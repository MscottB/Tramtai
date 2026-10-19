package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// opcionesPunto son las banderas de crear y editar un punto de acceso.
type opcionesPunto struct {
	puerto                            int
	instancia, agente, modelo, perfil string
	grupo, instrucciones              string
	contratos, permitidos, alias      []string
	forzar                            bool
	modoContexto, formato             string
	tiempo                            int
	presupuesto                       []string
}

func (o *opcionesPunto) registrar(cmd *cobra.Command) {
	f := cmd.Flags()
	f.IntVar(&o.puerto, "puerto", 0, idioma.T("puerto propio (0 = ruta /p/<nombre> en el puerto del servicio)"))
	f.StringVar(&o.instrucciones, "instrucciones", "", idioma.T("texto que se antepone a cada solicitud, por ejemplo \"Responde siempre en español\""))
	f.StringVar(&o.grupo, "grupo", "", idioma.T("grupo para ordenar los puntos y activarlos o desactivarlos juntos"))
	f.StringVar(&o.instancia, "instancia", "", idioma.T("instancia a la que pertenece (global, o la del proyecto con --local)"))
	f.StringVar(&o.agente, "agente", "", idioma.T("agente predeterminado del punto"))
	f.StringVar(&o.modelo, "modelo", "", idioma.T("modelo predeterminado del punto"))
	f.StringVar(&o.perfil, "perfil", "", idioma.T("perfil de modelo predeterminado del punto"))
	f.StringSliceVar(&o.contratos, "contratos", nil, idioma.T("API habilitadas: nativa, openai, anthropic (todas por defecto)"))
	f.StringSliceVar(&o.permitidos, "permitidos", nil, idioma.T("modelos permitidos: agente, agente/modelo o perfil/nombre"))
	f.StringSliceVar(&o.alias, "alias", nil, idioma.T("alias de modelos, por ejemplo gpt-4o-mini=claude/haiku"))
	f.BoolVar(&o.forzar, "forzar", false, idioma.T("ignora el modelo de la solicitud y usa el del punto"))
	f.StringVar(&o.modoContexto, "modo-contexto", "", idioma.T("modo de contexto del punto"))
	f.StringVar(&o.formato, "formato", "", idioma.T("formato de salida del punto"))
	f.IntVar(&o.tiempo, "tiempo", 0, idioma.T("tiempo límite en segundos del punto"))
	f.StringSliceVar(&o.presupuesto, "presupuesto", nil, idioma.T("límite periodo.unidad=valor (diario|mensual, peticiones|tokens|usd), repetible; 0 lo quita"))
}

// aplicarPresupuesto fija en p cada límite periodo.unidad=valor; 0 quita ese límite.
func aplicarPresupuesto(p *nucleo.Presupuesto, entradas []string) error {
	for _, entrada := range entradas {
		clave, valor, ok := strings.Cut(entrada, "=")
		periodo, unidad, _ := strings.Cut(clave, ".")
		limites := map[string]*nucleo.Limites{"diario": &p.Diario, "mensual": &p.Mensual}[periodo]
		numero, err := strconv.ParseFloat(valor, 64)
		if !ok || limites == nil || err != nil || !(numero >= 0) || math.IsInf(numero, 1) {
			return fmt.Errorf(idioma.T("presupuesto no válido %q: usa diario|mensual.peticiones|tokens|usd=valor"), entrada)
		}
		entero, errEntero := strconv.Atoi(valor)
		switch {
		case unidad == "usd":
			limites.USD = numero
		case unidad == "peticiones" && errEntero == nil:
			limites.Peticiones = entero
		case unidad == "tokens" && errEntero == nil:
			limites.Tokens = entero
		default:
			return fmt.Errorf(idioma.T("presupuesto no válido %q: usa diario|mensual.peticiones|tokens|usd=valor"), entrada)
		}
	}
	return nil
}

// aplicar copia al punto las banderas indicadas en la línea de comandos.
// El servicio valida el resultado al guardarlo, con las mismas reglas que la Web UI y la terminal.
func (o *opcionesPunto) aplicar(cmd *cobra.Command, p *contrato.ConfigPunto) error {
	cambio := cmd.Flags().Changed
	if cambio("puerto") {
		p.Puerto, p.RutaBase = o.puerto, ""
	}
	if cambio("instancia") {
		p.Instancia = o.instancia
	}
	if cambio("grupo") {
		p.Grupo = o.grupo
	}
	if cambio("instrucciones") {
		p.Instrucciones = strings.TrimSpace(o.instrucciones)
	}
	if cambio("agente") {
		p.Agente = o.agente
	}
	if cambio("modelo") {
		p.Modelo = o.modelo
	}
	if cambio("perfil") {
		p.PerfilModelo = o.perfil
	}
	if cambio("contratos") {
		p.Contratos = o.contratos
	}
	if cambio("permitidos") {
		p.ModelosPermitidos = o.permitidos
	}
	if cambio("forzar") {
		p.ForzarModelo = o.forzar
	}
	if cambio("alias") {
		p.AliasModelos = map[string]string{}
		for _, par := range o.alias {
			origen, destino, ok := strings.Cut(par, "=")
			if !ok || origen == "" || destino == "" {
				return fmt.Errorf(idioma.T("alias no válido %q: usa nombre=agente/modelo"), par)
			}
			p.AliasModelos[origen] = destino
		}
	}
	if cambio("modo-contexto") || cambio("formato") || cambio("tiempo") {
		if p.Ejecucion == nil {
			p.Ejecucion = &contrato.EjecucionPunto{}
		}
		if cambio("modo-contexto") {
			p.Ejecucion.ModoContexto = o.modoContexto
		}
		if cambio("formato") {
			p.Ejecucion.FormatoSalida = o.formato
		}
		if cambio("tiempo") {
			p.Ejecucion.TiempoLimiteSegundos = o.tiempo
		}
	}
	if cambio("presupuesto") {
		if p.Presupuesto == nil {
			p.Presupuesto = &nucleo.Presupuesto{}
		}
		if err := aplicarPresupuesto(p.Presupuesto, o.presupuesto); err != nil {
			return err
		}
	}
	return nil
}

// servicioAcceso conecta con el servicio o lo inicia en segundo plano: tramtai acceso edita las APIs por su API, con
// las mismas reglas que la Web UI y la terminal. Las pruebas lo cambian por un servicio en memoria.
var servicioAcceso = asegurarServicio

// acceso es lo que comparten los subcomandos: el cliente y el archivo elegido (el global o, con --local, el
// .tramtai/local.yaml del proyecto, cuya instancia se activa para ver sus puntos).
type acceso struct {
	cmd      *cobra.Command
	cli      *cliente.Cliente
	ctx      context.Context
	proyecto string // instancia del proyecto con --local; vacío, el archivo global
}

func abrirAcceso(cmd *cobra.Command, local bool) (*acceso, context.CancelFunc, error) {
	if local && raizProyecto() == "" {
		return nil, nil, errors.New(idioma.T("--local exige estar en un proyecto (.tramtai/config.yaml)"))
	}
	cli, err := servicioAcceso(cmd)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancelar := contexto()
	a := &acceso{cmd: cmd, cli: cli, ctx: ctx}
	if local {
		activacion, err := cli.Activar(ctx, "proyecto", raizProyecto())
		if err != nil {
			cancelar()
			return nil, nil, err
		}
		a.proyecto = activacion.Instancia.ID
	}
	return a, cancelar, nil
}

// esLocal indica si el archivo de un punto es el .tramtai/local.yaml de un proyecto.
func esLocal(origen string) bool {
	return strings.HasSuffix(filepath.ToSlash(origen), ".tramtai/local.yaml")
}

// puntos son los puntos del archivo elegido.
func (a *acceso) puntos() ([]contrato.EstadoPunto, error) {
	e, err := a.cli.Estado(a.ctx)
	if err != nil {
		return nil, err
	}
	var lista []contrato.EstadoPunto
	for _, p := range e.Puntos {
		if a.proyecto == "" && !esLocal(p.Origen) || a.proyecto != "" && esLocal(p.Origen) && p.Instancia == a.proyecto {
			lista = append(lista, p)
		}
	}
	return lista, nil
}

func (a *acceso) buscar(nombre string) (contrato.EstadoPunto, error) {
	lista, err := a.puntos()
	if err != nil {
		return contrato.EstadoPunto{}, err
	}
	i := slices.IndexFunc(lista, func(p contrato.EstadoPunto) bool { return p.Nombre == nombre })
	if i < 0 {
		return contrato.EstadoPunto{}, fmt.Errorf(idioma.T("no existe el punto de acceso %s en este archivo"), nombre)
	}
	return lista[i], nil
}

// instanciaNueva es donde se crea un punto: la del proyecto con --local; si no, la global, que lo guarda en la
// configuración global (con su instancia si se indicó --instancia).
func (a *acceso) instanciaNueva() string {
	if a.proyecto != "" {
		return a.proyecto
	}
	return "global"
}

// guardado cuenta dónde quedó el cambio y lista los puntos, como tras recargar.
func (a *acceso) guardado(origen string) error {
	if origen != "" {
		fmt.Fprintln(a.cmd.OutOrStdout(), idioma.Tf("guardado en %s", origen))
	}
	e, err := a.cli.Estado(a.ctx)
	if err != nil {
		return err
	}
	imprimirPuntos(a.cmd.OutOrStdout(), e.Puntos)
	return nil
}

func comandoAcceso() *cobra.Command {
	grupo := &cobra.Command{Use: "acceso", Short: idioma.T("Puntos de acceso: API adicionales con su puerto o ruta, token y modelos")}
	var local bool
	grupo.PersistentFlags().BoolVar(&local, "local", false, idioma.T("usa .tramtai/local.yaml del proyecto actual en lugar de la configuración global"))
	con := func(f func(a *acceso, args []string) error) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			a, cancelar, err := abrirAcceso(cmd, local)
			if err != nil {
				return err
			}
			defer cancelar()
			return f(a, args)
		}
	}

	listar := &cobra.Command{Use: "listar", Short: idioma.T("Lista los puntos de acceso y su estado"),
		RunE: con(func(a *acceso, _ []string) error { return a.guardado("") })}

	var crearOpc, editarOpc opcionesPunto
	crear := &cobra.Command{Use: "crear <nombre>", Short: idioma.T("Crea un punto de acceso y su token"), Args: cobra.ExactArgs(1),
		RunE: con(func(a *acceso, args []string) error {
			if _, err := a.buscar(args[0]); err == nil {
				return fmt.Errorf(idioma.T("el punto de acceso %s ya existe: usa tramtai acceso editar"), args[0])
			}
			p := contrato.ConfigPunto{Nombre: args[0]}
			if err := crearOpc.aplicar(a.cmd, &p); err != nil {
				return err
			}
			if a.proyecto != "" {
				p.Instancia = "" // un punto de .tramtai/local.yaml es siempre de su proyecto
			} else if _, err := a.cli.Activar(a.ctx, "global", ""); err != nil { // se crea desde la global, como en la Web UI
				return err
			}
			_, token, err := a.cli.CrearPunto(a.ctx, a.instanciaNueva(), p)
			if err != nil {
				return err
			}
			creado, _ := a.buscar(p.Nombre)
			if err := a.guardado(creado.Origen); err != nil {
				return err
			}
			fmt.Fprintf(a.cmd.OutOrStdout(), idioma.T("clave nueva de %s (guárdala: no se volverá a mostrar):\n%s\n"), p.Nombre, token)
			return nil
		})}
	crearOpc.registrar(crear)
	editarCmd := &cobra.Command{Use: "editar <nombre>", Short: idioma.T("Cambia las opciones indicadas de un punto de acceso"), Args: cobra.ExactArgs(1),
		RunE: con(func(a *acceso, args []string) error {
			e, err := a.buscar(args[0])
			if err != nil {
				return err
			}
			p, err := a.cli.ConfigDePunto(a.ctx, e.Instancia, e.Nombre)
			if err != nil {
				return err
			}
			if err := editarOpc.aplicar(a.cmd, &p); err != nil {
				return err
			}
			if err := a.cli.ReemplazarPunto(a.ctx, e.Instancia, p); err != nil {
				return err
			}
			return a.guardado(e.Origen)
		})}
	editarOpc.registrar(editarCmd)
	activo := func(uso string, valor bool) *cobra.Command {
		var grupoPuntos string
		c := &cobra.Command{Use: uso + " <nombre>... | --grupo <grupo>",
			Short: map[bool]string{true: idioma.T("Activa uno o varios puntos de acceso sin reiniciar el servicio"),
				false: idioma.T("Desactiva uno o varios puntos de acceso sin reiniciar el servicio")}[valor],
			RunE: con(func(a *acceso, args []string) error {
				if (len(args) == 0) == (grupoPuntos == "") {
					return errors.New(idioma.T("indica los nombres o --grupo, no ambos"))
				}
				var elegidos []contrato.EstadoPunto
				if grupoPuntos != "" {
					lista, err := a.puntos()
					if err != nil {
						return err
					}
					for _, p := range lista {
						if p.Grupo == grupoPuntos {
							elegidos = append(elegidos, p)
						}
					}
					if len(elegidos) == 0 {
						return fmt.Errorf(idioma.T("ningún punto de acceso del grupo %s en este archivo"), grupoPuntos)
					}
				}
				for _, nombre := range args {
					p, err := a.buscar(nombre)
					if err != nil {
						return err
					}
					elegidos = append(elegidos, p)
				}
				for _, p := range elegidos {
					if err := a.cli.ActivarPunto(a.ctx, p.Instancia, p.Nombre, valor); err != nil {
						return err
					}
				}
				return a.guardado(elegidos[0].Origen)
			})}
		c.Flags().StringVar(&grupoPuntos, "grupo", "", idioma.T("todos los puntos de ese grupo"))
		return c
	}
	eliminar := &cobra.Command{Use: "eliminar <nombre>", Short: idioma.T("Elimina un punto de acceso"), Args: cobra.ExactArgs(1),
		RunE: con(func(a *acceso, args []string) error {
			p, err := a.buscar(args[0])
			if err != nil {
				return err
			}
			if err := a.cli.EliminarPunto(a.ctx, p.Instancia, p.Nombre); err != nil {
				return err
			}
			return a.guardado(p.Origen)
		})}
	var rotar bool
	token := &cobra.Command{Use: "token <nombre>", Short: idioma.T("Regenera con --rotar la clave del punto de acceso; solo se ve entonces"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Solo se guarda la huella de la clave: sin --rotar no hay nada que mostrar, ni hace falta el servicio.
			if !rotar {
				return errors.New(idioma.Tf("la clave de %s solo se muestra al crearla o regenerarla (se guarda su huella): tramtai acceso token %s --rotar", args[0], args[0]))
			}
			return con(func(a *acceso, args []string) error {
				p, err := a.buscar(args[0])
				if err != nil {
					return err
				}
				valor, err := a.cli.RotarTokenPunto(a.ctx, p.Instancia, p.Nombre)
				if err != nil {
					return err
				}
				fmt.Fprintf(a.cmd.OutOrStdout(), idioma.T("clave nueva de %s (guárdala: no se volverá a mostrar):\n%s\n"), p.Nombre, valor)
				return nil
			})(cmd, args)
		}}
	token.Flags().BoolVar(&rotar, "rotar", false, idioma.T("genera un token nuevo; el anterior deja de valer"))
	var instanciaPresupuesto string
	reiniciar := &cobra.Command{Use: "reiniciar-presupuesto [nombre]", Args: cobra.MaximumNArgs(1),
		Short: idioma.T("Pone a cero el consumo del día y del mes de un punto de acceso, o de la instancia sin nombre"),
		RunE: func(cmd *cobra.Command, args []string) error {
			nombre := ""
			if len(args) == 1 {
				nombre = args[0]
			}
			return reiniciarPresupuesto(cmd, instanciaPresupuesto, nombre)
		}}
	reiniciar.Flags().StringVar(&instanciaPresupuesto, "instancia", "", idioma.T("instancia cuyo presupuesto se reinicia (global por defecto)"))
	grupo.AddCommand(listar, crear, editarCmd, activo("activar", true), activo("desactivar", false), eliminar, token, reiniciar)
	return grupo
}

// reiniciarPresupuesto pone a cero el consumo en curso de un punto (en su instancia) o de una instancia. El consumo
// está en la base del servicio: sin servicio en marcha no hay nada que reiniciar.
func reiniciarPresupuesto(cmd *cobra.Command, instancia, nombre string) error {
	c, err := conectar()
	if err != nil {
		return errors.New(idioma.T("el servicio no está en marcha: el consumo se guarda en su base (inícialo con tramtai servicio iniciar)"))
	}
	ctx, cancelar := contexto()
	defer cancelar()
	if nombre != "" {
		e, err := c.Estado(ctx)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(e.Puntos, func(p contrato.EstadoPunto) bool { return p.Nombre == nombre })
		if i < 0 {
			return fmt.Errorf(idioma.T("no existe el punto de acceso %s en el servicio"), nombre)
		}
		instancia = e.Puntos[i].Instancia
	}
	if err := c.ReiniciarPresupuesto(ctx, instancia, nombre); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), idioma.T("consumo reiniciado"))
	return nil
}
