package copilot

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"tramtai/internal/adaptadores"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
)

// variableClave es el token de GitHub (fine-grained, con el permiso Copilot Requests) para usar Copilot sin la sesión
// de copilot login. El token clásico de gh (ghp_) no vale.
const variableClave = "COPILOT_GITHUB_TOKEN"

// Copilot ejecuta "copilot --output-format json" sin herramientas, sin MCP y con un COPILOT_HOME privado por
// ejecución. El texto llega en vivo y el modelo efectivo se informa (también el que elige auto).
type Copilot struct {
	Programa      string            // comandos_agentes.copilot
	Autenticacion *adaptadores.Modo // clave_api (predeterminada) o suscripcion (la sesión de copilot login)
}

// Detectar implementa Detectable.
func (c Copilot) Detectar(ctx context.Context) adaptadores.Estado {
	e := adaptadores.Estado{Agente: "copilot", Autenticacion: c.Autenticacion.Valor(),
		Terminos: idioma.T("términos de GitHub Copilot: tu cuenta y tu plan; cada petición gasta créditos")}
	if c.Autenticacion.Valor() == adaptadores.AutenticacionSuscripcion {
		if _, err := sesion(); err != nil {
			e.Problema, e.ProblemaCodigo = problemaSesion(err), adaptadores.ProblemaFaltaClave
		}
	} else if adaptadores.ClaveEntorno(variableClave) == "" {
		e.Problema, e.ProblemaCodigo = idioma.T("falta COPILOT_GITHUB_TOKEN"), adaptadores.ProblemaFaltaClave
	}
	ruta, err := programa(c.Programa)
	if err != nil {
		e.Problema, e.ProblemaCodigo = err.Error(), adaptadores.ProblemaNoInstalado
		return e
	}
	// «GitHub Copilot CLI 1.0.98.» queda en «1.0.98»: el nombre ya se muestra al lado.
	e.Ruta, e.Version = ruta, strings.TrimSuffix(strings.TrimPrefix(adaptadores.VersionDe(ctx, ruta), "GitHub Copilot CLI "), ".")
	return e
}

// Ejecutar implementa Adaptador.
func (c Copilot) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	ruta, err := programa(c.Programa)
	if err != nil {
		return adaptadores.Resultado{}, err
	}
	raiz, espacio, err := adaptadores.EspacioTemporal()
	if err != nil {
		return adaptadores.Resultado{}, err
	}
	defer os.RemoveAll(raiz)
	// Un COPILOT_HOME privado: ni la configuración, ni los plugins, ni los MCP del usuario, y las sesiones que guarda
	// Copilot se borran al terminar.
	home := filepath.Join(raiz, "copilot-home")
	if err := os.Mkdir(home, 0o700); err != nil {
		return adaptadores.Resultado{}, err
	}
	extra := []string{"COPILOT_HOME=" + home}
	var clave string
	switch c.Autenticacion.Valor() {
	case adaptadores.AutenticacionSuscripcion:
		config, err := sesion()
		if err != nil {
			mensaje := problemaSesion(err)
			if sinSesion(err) {
				mensaje = idioma.T("no hay sesión de Copilot: inicia sesión con copilot login o configura adaptadores.copilot.autenticacion: clave_api")
			}
			return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoAutenticacionAgente, mensaje)
		}
		if err := os.WriteFile(filepath.Join(home, "config.json"), config, 0o600); err != nil {
			return adaptadores.Resultado{}, err
		}
	case adaptadores.AutenticacionClaveAPI, "":
		if clave = adaptadores.ClaveEntorno(variableClave); clave == "" {
			return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoAutenticacionAgente,
				idioma.T("falta COPILOT_GITHUB_TOKEN; o configura adaptadores.copilot.autenticacion: suscripcion para usar la sesión de copilot login"))
		}
		extra = append(extra, variableClave+"="+clave)
	default:
		return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("autenticación desconocida: %s", c.Autenticacion.Valor()))
	}

	archivoUso := filepath.Join(raiz, "uso.json")
	sistema, prompt := adaptadores.Separar(s)
	formato := adaptadores.FormatoEnInstrucciones(s.Salida)
	if formato != "" {
		sistema = strings.TrimSpace(sistema + "\n\n" + formato)
	}
	// Salvaguarda: si una versión de Copilot dejara de respetar la lista vacía de herramientas, la ejecución se corta en
	// cuanto pida una (procesos.Ejecutar no corta el proceso porque la lectura devuelva un error: hace falta cancelar).
	ctxCLI, cancelar := context.WithCancel(ctx)
	defer cancelar()
	e := estado{detener: cancelar}
	r, err := procesos.Ejecutar(ctxCLI, procesos.Orden{Programa: ruta, Argumentos: argumentos(s, archivoUso),
		Directorio: adaptadores.DirectorioTrabajo(s, espacio), Entorno: entorno(extra),
		Entrada: adaptadores.EnmarcarSistema(sistema, prompt)}, func(linea string) error { return e.leer(linea, emitir) })
	if e.herramienta && ctx.Err() == nil {
		return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoErrorAgente,
			idioma.T("copilot intentó usar una herramienta y se detuvo: Tramtai lo ejecuta sin herramientas"))
	}
	if err != nil {
		return adaptadores.Resultado{}, err
	}
	if !e.terminado || e.codigo != 0 {
		texto := adaptadores.RedactarCon(strings.TrimSpace(strings.Join(e.errores, "\n")+"\n"+r.Errores), clave)
		switch {
		case texto == "" && !e.terminado:
			return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoErrorProtocoloAgente, idioma.T("copilot terminó sin result"))
		case texto == "":
			return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoErrorAgente, idioma.Tf("copilot terminó con el código %d", e.codigo))
		}
		return adaptadores.Resultado{}, nucleo.NuevoError(adaptadores.Clasificar(texto, 0), texto)
	}
	if e.creditos > 0 {
		if err := emitir(adaptadores.Evento{Tipo: "aviso", Datos: idioma.Tf("Créditos de tu plan de Copilot gastados (premium requests): %g", e.creditos)}); err != nil {
			return adaptadores.Resultado{}, err
		}
	}
	salida := strings.Join(e.mensajes, "\n\n")
	if formato != "" {
		salida = adaptadores.SinBloqueDeCodigo(salida)
	}
	return adaptadores.Resultado{Salida: salida, Uso: leerUso(archivoUso), ModeloEfectivo: e.modelo}, nil
}

// argumentos son los que pasaron el canario: sin herramientas («none» no es ninguna, así que la lista queda vacía),
// sin la carpeta temporal, sin MCP, sin preguntas y sin actualizarse. El mensaje llega por stdin.
func argumentos(s nucleo.SolicitudEjecucion, archivoUso string) []string {
	modelo := cmp.Or(s.Modelo, "auto")
	args := []string{"--output-format", "json", "--model", modelo, "--available-tools", "none", "--disallow-temp-dir",
		"--disable-builtin-mcps", "--no-ask-user", "--no-auto-update", "--usage-output-file", archivoUso}
	if !adaptadores.Consciente(s) {
		args = append(args, "--no-custom-instructions") // en modo consciente carga las instrucciones del proyecto
	}
	// auto no admite esfuerzo: Copilot falla con «Model "auto" does not support reasoning effort».
	if v := adaptadores.ValorEsfuerzo(valoresEsfuerzo, s.Esfuerzo); v != "" && !slices.Contains(Ficha.ModelosSinEsfuerzo, s.Modelo) {
		args = append(args, "--reasoning-effort", v)
	}
	return args
}

// entorno quita, además de las credenciales, toda variable COPILOT_* heredada (un proveedor BYOK o un COPILOT_HOME
// ajeno), salvo las que pone Tramtai.
func entorno(extra []string) []string {
	return slices.DeleteFunc(adaptadores.EntornoLimpio(nil, extra...), func(par string) bool {
		return strings.HasPrefix(strings.ToUpper(par), "COPILOT_") && !slices.Contains(extra, par)
	})
}

var errSinSesion = errors.New("sin sesión de Copilot")

// sinSesion distingue «no hay sesión» de un config.json que no se pudo leer o entender.
func sinSesion(err error) bool { return errors.Is(err, errSinSesion) || errors.Is(err, fs.ErrNotExist) }

func problemaSesion(err error) string {
	if sinSesion(err) {
		return idioma.T("no hay sesión de Copilot: inicia sesión con copilot login")
	}
	return idioma.Tf("no se pudo leer la sesión de Copilot: %v", err)
}

// sesion devuelve el config.json para el COPILOT_HOME privado: solo la cuenta con la que el usuario inició sesión
// (loggedInUsers y lastLoggedInUser, sin secretos: el token está en el almacén del sistema).
func sesion() ([]byte, error) {
	dir := os.Getenv("COPILOT_HOME")
	if dir == "" {
		inicio, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(inicio, ".copilot")
	}
	archivo := filepath.Join(dir, "config.json")
	datos, err := os.ReadFile(archivo)
	if err != nil {
		return nil, err
	}
	// Copilot escribe el archivo con comentarios de línea al principio (JSONC).
	lineas := slices.DeleteFunc(strings.Split(string(datos), "\n"), func(l string) bool { return strings.HasPrefix(strings.TrimSpace(l), "//") })
	var config struct {
		Usuarios []json.RawMessage `json:"loggedInUsers"`
		Ultimo   json.RawMessage   `json:"lastLoggedInUser"`
	}
	if err := json.Unmarshal([]byte(strings.Join(lineas, "\n")), &config); err != nil {
		return nil, fmt.Errorf("%s: %w", archivo, err)
	}
	if len(config.Usuarios) == 0 {
		return nil, errSinSesion
	}
	privado := map[string]any{"loggedInUsers": config.Usuarios}
	if len(config.Ultimo) > 0 {
		privado["lastLoggedInUser"] = config.Ultimo
	}
	return json.Marshal(privado)
}

// programa resuelve el CLI. En Windows, npm instala copilot.cmd, un envoltorio que no se ejecuta por seguridad: se usa
// el copilot.exe nativo que trae su paquete de plataforma.
func programa(nombre string) (string, error) {
	ruta, err := adaptadores.ResolverPrograma(nombre)
	if err == nil || runtime.GOOS != "windows" {
		return ruta, err
	}
	envoltorio, errBusqueda := exec.LookPath(nombre)
	if errBusqueda != nil || !strings.EqualFold(filepath.Ext(envoltorio), ".cmd") {
		return "", err
	}
	plataforma := "copilot-win32-" + map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	modulos := filepath.Join(filepath.Dir(envoltorio), "node_modules", "@github")
	for _, base := range []string{filepath.Join(modulos, "copilot", "node_modules", "@github"), modulos} {
		if exe := filepath.Join(base, plataforma, "copilot.exe"); esArchivo(exe) {
			return exe, nil
		}
	}
	return "", err
}

func esArchivo(ruta string) bool {
	info, err := os.Stat(ruta)
	return err == nil && info.Mode().IsRegular()
}

// estado es lo que se va leyendo del JSONL de Copilot.
type estado struct {
	mensajes    []string
	errores     []string // los que Copilot informa como eventos, además de stderr
	modelo      string
	terminado   bool
	codigo      int
	creditos    float64 // los multiplicadores de los planes de pago son fraccionarios (0,33×…)
	herramienta bool    // pidió una herramienta: se corta la ejecución
	detener     func()
}

func (e *estado) pararPorHerramienta() {
	e.herramienta = true
	if e.detener != nil {
		e.detener()
	}
}

func (e *estado) leer(linea string, emitir func(adaptadores.Evento) error) error {
	if e.herramienta {
		return nil // tras una herramienta no se emite nada: el texto podría llevar lo que obtuvo
	}
	var ev eventoCopilot
	if json.Unmarshal([]byte(linea), &ev) != nil {
		return nil // línea informativa no JSON
	}
	switch ev.Type {
	case "assistant.message_delta":
		if ev.Data.DeltaContent != "" {
			return emitir(adaptadores.Evento{Tipo: "texto", Datos: ev.Data.DeltaContent})
		}
	case "assistant.message":
		if len(ev.Data.ToolRequests) > 0 {
			e.pararPorHerramienta()
			return nil
		}
		if ev.Data.Content != "" {
			e.mensajes = append(e.mensajes, ev.Data.Content)
		}
		e.modelo = cmp.Or(ev.Data.Model, e.modelo)
	case "session.auto_mode_resolved":
		e.modelo = cmp.Or(e.modelo, ev.Data.ChosenModel)
	case "result":
		e.terminado, e.codigo = true, ev.ExitCode
		// El uso va aparte: un cambio en su forma no debe hacer perder el resultado.
		var uso struct {
			PremiumRequests float64 `json:"premiumRequests"`
		}
		if json.Unmarshal(ev.Usage, &uso) == nil {
			e.creditos = uso.PremiumRequests
		}
	default:
		switch {
		case strings.HasPrefix(ev.Type, "tool."):
			e.pararPorHerramienta()
		case strings.HasSuffix(ev.Type, "error") && ev.Data.Message != "":
			e.errores = append(e.errores, ev.Data.Message)
		}
	}
	return nil // los demás tipos son informativos
}

type eventoCopilot struct {
	Type string `json:"type"`
	Data struct {
		DeltaContent string            `json:"deltaContent"`
		Content      string            `json:"content"`
		Model        string            `json:"model"`
		ChosenModel  string            `json:"chosenModel"`
		Message      string            `json:"message"`
		ToolRequests []json.RawMessage `json:"toolRequests"`
	} `json:"data"`
	ExitCode int             `json:"exitCode"`
	Usage    json.RawMessage `json:"usage"`
}

// leerUso suma los tokens de cada modelo del archivo de --usage-output-file; sin archivo, el uso queda en cero.
func leerUso(archivo string) nucleo.Uso {
	var datos struct {
		Modelos map[string]struct {
			Usage struct {
				Entrada        int `json:"inputTokens"`
				Salida         int `json:"outputTokens"`
				CacheLectura   int `json:"cacheReadTokens"`
				CacheEscritura int `json:"cacheWriteTokens"`
			} `json:"usage"`
		} `json:"modelMetrics"`
	}
	contenido, err := os.ReadFile(archivo)
	if err != nil || json.Unmarshal(contenido, &datos) != nil {
		return nucleo.Uso{}
	}
	var uso nucleo.Uso
	for _, m := range datos.Modelos {
		uso.TokensEntrada += m.Usage.Entrada + m.Usage.CacheLectura + m.Usage.CacheEscritura
		uso.TokensSalida += m.Usage.Salida
	}
	return uso
}
