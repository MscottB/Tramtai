package main

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/servicio"
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
func (o *opcionesPunto) aplicar(cmd *cobra.Command, p *servicio.ConfigPunto) error {
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
			p.Ejecucion = &servicio.EjecucionPunto{}
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
	normalizado, err := p.Normalizar("global", 0)
	p.Presupuesto = normalizado.Presupuesto // sin límites queda sin presupuesto
	return err
}

func comandoAcceso() *cobra.Command {
	grupo := &cobra.Command{Use: "acceso", Short: idioma.T("Puntos de acceso: API adicionales con su puerto o ruta, token y modelos")}
	var local bool
	grupo.PersistentFlags().BoolVar(&local, "local", false, idioma.T("usa .tramtai/local.yaml del proyecto actual en lugar de la configuración global"))
	archivo := func() (string, nucleo.Origen, error) {
		if !local {
			ruta, err := nucleo.RutaGlobal()
			return ruta, nucleo.OrigenGlobal, err
		}
		raiz := raizProyecto()
		if raiz == "" {
			return "", 0, errors.New(idioma.T("--local exige estar en un proyecto (.tramtai/config.yaml)"))
		}
		return filepath.Join(raiz, ".tramtai", "local.yaml"), nucleo.OrigenLocal, nil
	}
	editar := func(cmd *cobra.Command, f func([]servicio.ConfigPunto) ([]servicio.ConfigPunto, error)) error {
		ruta, origen, err := archivo()
		if err != nil {
			return err
		}
		if err := servicio.EditarPuntos(ruta, origen, f); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), idioma.Tf("guardado en %s", ruta))
		recargarSiActivo(cmd)
		return nil
	}
	buscar := func(lista []servicio.ConfigPunto, nombre string) (int, error) {
		i := slices.IndexFunc(lista, func(p servicio.ConfigPunto) bool { return p.Nombre == nombre })
		if i < 0 {
			return i, fmt.Errorf(idioma.T("no existe el punto de acceso %s en este archivo"), nombre)
		}
		return i, nil
	}

	listar := &cobra.Command{Use: "listar", Short: idioma.T("Lista los puntos de acceso y su estado"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if c, err := conectar(); err == nil {
				ctx, cancelar := contexto()
				defer cancelar()
				e, err := c.Estado(ctx)
				if err != nil {
					return err
				}
				imprimirPuntos(cmd.OutOrStdout(), e.Puntos)
				return nil
			}
			ruta, origen, err := archivo()
			if err != nil {
				return err
			}
			capa, err := nucleo.CargarCapa(ruta, origen)
			if err != nil {
				return err
			}
			lista, _ := capa.Valores["puntos_acceso"].([]any)
			puntos, err := servicio.LeerPuntos(lista)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), idioma.Tf("(servicio inactivo; puntos definidos en %s)", ruta))
			for _, p := range puntos {
				fmt.Fprintf(cmd.OutOrStdout(), idioma.T("  %-18s puerto %-6d activo %v\n"), p.Nombre, p.Puerto, p.EstaActivo())
			}
			return nil
		}}

	var crearOpc, editarOpc opcionesPunto
	crear := &cobra.Command{Use: "crear <nombre>", Short: idioma.T("Crea un punto de acceso y su token"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := servicio.ConfigPunto{Nombre: args[0]}
			if err := crearOpc.aplicar(cmd, &p); err != nil {
				return err
			}
			if err := editar(cmd, func(lista []servicio.ConfigPunto) ([]servicio.ConfigPunto, error) {
				if _, err := buscar(lista, p.Nombre); err == nil {
					return nil, fmt.Errorf(idioma.T("el punto de acceso %s ya existe: usa tramtai acceso editar"), p.Nombre)
				}
				return append(lista, p), nil
			}); err != nil {
				return err
			}
			return mostrarTokenPunto(cmd, p.Nombre, true) // la clave se genera y se muestra ahora, una sola vez
		}}
	crearOpc.registrar(crear)
	editarCmd := &cobra.Command{Use: "editar <nombre>", Short: idioma.T("Cambia las opciones indicadas de un punto de acceso"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return editar(cmd, func(lista []servicio.ConfigPunto) ([]servicio.ConfigPunto, error) {
				i, err := buscar(lista, args[0])
				if err != nil {
					return nil, err
				}
				return lista, editarOpc.aplicar(cmd, &lista[i])
			})
		}}
	editarOpc.registrar(editarCmd)
	activo := func(uso string, valor bool) *cobra.Command {
		var grupoPuntos string
		c := &cobra.Command{Use: uso + " <nombre>... | --grupo <grupo>",
			Short: map[bool]string{true: idioma.T("Activa uno o varios puntos de acceso sin reiniciar el servicio"),
				false: idioma.T("Desactiva uno o varios puntos de acceso sin reiniciar el servicio")}[valor],
			RunE: func(cmd *cobra.Command, args []string) error {
				if (len(args) == 0) == (grupoPuntos == "") {
					return errors.New(idioma.T("indica los nombres o --grupo, no ambos"))
				}
				return editar(cmd, func(lista []servicio.ConfigPunto) ([]servicio.ConfigPunto, error) {
					if grupoPuntos != "" {
						n := 0
						for i := range lista {
							if lista[i].Grupo == grupoPuntos {
								lista[i].Activo, n = &valor, n+1
							}
						}
						if n == 0 {
							return nil, fmt.Errorf(idioma.T("ningún punto de acceso del grupo %s en este archivo"), grupoPuntos)
						}
						return lista, nil
					}
					for _, nombre := range args {
						i, err := buscar(lista, nombre)
						if err != nil {
							return nil, err
						}
						lista[i].Activo = &valor
					}
					return lista, nil
				})
			}}
		c.Flags().StringVar(&grupoPuntos, "grupo", "", idioma.T("todos los puntos de ese grupo"))
		return c
	}
	eliminar := &cobra.Command{Use: "eliminar <nombre>", Short: idioma.T("Elimina un punto de acceso"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ruta, origen, err := archivo()
			if err != nil {
				return err
			}
			if err := servicio.EditarPuntos(ruta, origen, func(lista []servicio.ConfigPunto) ([]servicio.ConfigPunto, error) {
				i, err := buscar(lista, args[0])
				if err != nil {
					return nil, err
				}
				return slices.Delete(lista, i, i+1), nil
			}); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), idioma.Tf("guardado en %s", ruta))
			// La clave se borra antes de recargar: así el servicio sabe que el punto se eliminó, y otro con este nombre
			// no hereda ni la clave ni el consumo.
			if datos, err := dirDatos(); err == nil {
				servicio.EliminarTokenPunto(datos, args[0])
			}
			recargarSiActivo(cmd)
			return nil
		}}
	var rotar bool
	token := &cobra.Command{Use: "token <nombre>", Short: idioma.T("Regenera con --rotar la clave del punto de acceso; solo se ve entonces"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return mostrarTokenPunto(cmd, args[0], rotar) }}
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

// mostrarTokenPunto genera la clave de un punto y la muestra esta única vez: solo se guarda su huella, así que sin
// --rotar no hay nada que mostrar.
func mostrarTokenPunto(cmd *cobra.Command, nombre string, rotar bool) error {
	if !servicio.NombrePuntoValido(nombre) {
		return errors.New(idioma.T("nombre no válido (minúsculas, números, _ y -; hasta 40)"))
	}
	if !rotar {
		return errors.New(idioma.Tf("la clave de %s solo se muestra al crearla o regenerarla (se guarda su huella): tramtai acceso token %s --rotar", nombre, nombre))
	}
	datos, err := dirDatos()
	if err != nil {
		return err
	}
	valor, err := servicio.RotarTokenPunto(servicio.RutaTokenPunto(datos, nombre))
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), idioma.T("clave nueva de %s (guárdala: no se volverá a mostrar):\n%s\n"), nombre, valor)
	recargarSiActivo(cmd)
	return nil
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
		i := slices.IndexFunc(e.Puntos, func(p servicio.EstadoPunto) bool { return p.Nombre == nombre })
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

// recargarSiActivo aplica los cambios en el servicio en marcha; si no lo está, se aplicarán al iniciarlo.
func recargarSiActivo(cmd *cobra.Command) {
	c, err := conectar()
	if err != nil {
		fmt.Fprintln(cmd.OutOrStdout(), idioma.T("el servicio no está en marcha: el cambio se aplicará al iniciarlo"))
		return
	}
	ctx, cancelar := contexto()
	defer cancelar()
	puntos, err := c.Recargar(ctx)
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), idioma.T("no se pudo recargar:"), err)
		return
	}
	imprimirPuntos(cmd.OutOrStdout(), puntos)
}
