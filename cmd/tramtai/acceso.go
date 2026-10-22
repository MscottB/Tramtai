package main

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	f.StringVar(&o.instrucciones, "instrucciones", "", idioma.T("texto que se antepone a cada petición, por ejemplo «Responde siempre en español»"))
	f.StringVar(&o.grupo, "grupo", "", idioma.T("grupo para ordenar las APIs y activarlas o detenerlas juntas"))
	f.StringVar(&o.instancia, "instancia", "", idioma.T("espacio al que pertenece (global, o el del proyecto con --local)"))
	f.StringVar(&o.agente, "agente", "", idioma.T("agente predeterminado de la API"))
	f.StringVar(&o.modelo, "modelo", "", idioma.T("modelo predeterminado de la API"))
	f.StringVar(&o.perfil, "perfil", "", idioma.T("perfil de modelo predeterminado de la API"))
	f.StringSliceVar(&o.contratos, "contratos", nil, idioma.T("API habilitadas: nativa, openai, anthropic (todas por defecto)"))
	f.StringSliceVar(&o.permitidos, "permitidos", nil, idioma.T("modelos permitidos: agente, agente/modelo o perfil/nombre"))
	f.StringSliceVar(&o.alias, "alias", nil, idioma.T("alias de modelos, por ejemplo gpt-4o-mini=claude/haiku"))
	f.BoolVar(&o.forzar, "forzar", false, idioma.T("ignora el modelo que pide la petición y usa el de la API"))
	f.StringVar(&o.modoContexto, "modo-contexto", "", idioma.T("modo de contexto de la API"))
	f.StringVar(&o.formato, "formato", "", idioma.T("formato de salida de la API"))
	f.IntVar(&o.tiempo, "tiempo", 0, idioma.T("tiempo límite en segundos de la API"))
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

// servicioAcceso conecta con el servicio o lo inicia en segundo plano: tramtai acceso guarda las APIs a través de él,
// con las mismas reglas que la Web UI y la terminal. Las pruebas lo cambian por un servicio en memoria.
var servicioAcceso = asegurarServicio

// guardado cuenta dónde quedó el cambio y lista los puntos del servicio tras recargar.
func guardado(cmd *cobra.Command, g cliente.GuardadoEnArchivo) {
	fmt.Fprintln(cmd.OutOrStdout(), idioma.Tf("guardado en %s", g.Archivo))
	imprimirPuntos(cmd.OutOrStdout(), g.Puntos)
}

func comandoAcceso() *cobra.Command {
	grupo := &cobra.Command{Use: "acceso", Short: idioma.T("APIs por aplicación, cada una con su puerto o ruta, su clave y sus modelos")}
	var local bool
	grupo.PersistentFlags().BoolVar(&local, "local", false, idioma.T("usa .tramtai/local.yaml del proyecto actual en lugar de la configuración global"))
	// con conecta con el servicio y pasa el archivo elegido: "" es la configuración global; con --local, la raíz del
	// proyecto, cuyo .tramtai/local.yaml edita el servicio.
	con := func(f func(cmd *cobra.Command, cli *cliente.Cliente, ctx context.Context, proyecto string, args []string) error) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			proyecto := ""
			if local {
				if proyecto = raizProyecto(); proyecto == "" {
					return errors.New(idioma.T("--local exige estar en un proyecto (.tramtai/config.yaml)"))
				}
			}
			cli, err := servicioAcceso(cmd)
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			return f(cmd, cli, ctx, proyecto, args)
		}
	}
	buscar := func(ctx context.Context, cli *cliente.Cliente, proyecto, nombre string) (contrato.ConfigPunto, error) {
		_, lista, err := cli.PuntosEnArchivo(ctx, proyecto)
		if err != nil {
			return contrato.ConfigPunto{}, err
		}
		i := slices.IndexFunc(lista, func(p contrato.ConfigPunto) bool { return p.Nombre == nombre })
		if i < 0 {
			return contrato.ConfigPunto{}, fmt.Errorf(idioma.T("no existe la API %s en este archivo"), nombre)
		}
		return lista[i], nil
	}

	listar := &cobra.Command{Use: "listar", Short: idioma.T("Lista las APIs y su estado"),
		RunE: con(func(cmd *cobra.Command, cli *cliente.Cliente, ctx context.Context, _ string, _ []string) error {
			e, err := cli.Estado(ctx)
			if err != nil {
				return err
			}
			imprimirPuntos(cmd.OutOrStdout(), e.Puntos)
			return nil
		})}

	var crearOpc, editarOpc opcionesPunto
	crear := &cobra.Command{Use: "crear <nombre>", Short: idioma.T("Crea una API y su clave"), Args: cobra.ExactArgs(1),
		RunE: con(func(cmd *cobra.Command, cli *cliente.Cliente, ctx context.Context, proyecto string, args []string) error {
			p := contrato.ConfigPunto{Nombre: args[0]}
			if err := crearOpc.aplicar(cmd, &p); err != nil {
				return err
			}
			g, err := cli.CrearPuntoEnArchivo(ctx, proyecto, p)
			if err != nil {
				return err
			}
			guardado(cmd, g)
			// La clave se genera y se muestra ahora, una sola vez: solo se guarda su huella.
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("clave nueva de %s (guárdala: no se volverá a mostrar):\n%s\n"), p.Nombre, g.Token)
			return nil
		})}
	crearOpc.registrar(crear)
	editarCmd := &cobra.Command{Use: "editar <nombre>", Short: idioma.T("Cambia las opciones indicadas de una API"), Args: cobra.ExactArgs(1),
		RunE: con(func(cmd *cobra.Command, cli *cliente.Cliente, ctx context.Context, proyecto string, args []string) error {
			p, err := buscar(ctx, cli, proyecto, args[0])
			if err != nil {
				return err
			}
			if err := editarOpc.aplicar(cmd, &p); err != nil {
				return err
			}
			g, err := cli.GuardarPuntoEnArchivo(ctx, proyecto, p)
			if err != nil {
				return err
			}
			guardado(cmd, g)
			return nil
		})}
	editarOpc.registrar(editarCmd)
	activo := func(uso string, valor bool) *cobra.Command {
		var grupoPuntos string
		c := &cobra.Command{Use: uso + " <nombre>... | --grupo <grupo>",
			Short: map[bool]string{true: idioma.T("Activa una o varias APIs sin reiniciar el servicio"),
				false: idioma.T("Detiene una o varias APIs sin reiniciar el servicio")}[valor],
			RunE: func(cmd *cobra.Command, args []string) error {
				if (len(args) == 0) == (grupoPuntos == "") {
					return errors.New(idioma.T("indica los nombres o --grupo, no ambos"))
				}
				return con(func(cmd *cobra.Command, cli *cliente.Cliente, ctx context.Context, proyecto string, args []string) error {
					g, err := cli.ActivarPuntosEnArchivo(ctx, proyecto, args, grupoPuntos, valor)
					if err != nil {
						return err
					}
					guardado(cmd, g)
					return nil
				})(cmd, args)
			}}
		c.Flags().StringVar(&grupoPuntos, "grupo", "", idioma.T("todas las APIs de ese grupo"))
		return c
	}
	eliminar := &cobra.Command{Use: "eliminar <nombre>", Short: idioma.T("Elimina una API"), Args: cobra.ExactArgs(1),
		RunE: con(func(cmd *cobra.Command, cli *cliente.Cliente, ctx context.Context, proyecto string, args []string) error {
			g, err := cli.EliminarPuntoEnArchivo(ctx, proyecto, args[0])
			if err != nil {
				return err
			}
			guardado(cmd, g)
			return nil
		})}
	var rotar bool
	token := &cobra.Command{Use: "token <nombre>", Short: idioma.T("Regenera con --rotar la clave de la API; solo se ve entonces"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Solo se guarda la huella de la clave: sin --rotar no hay nada que mostrar, ni hace falta el servicio.
			if !rotar {
				return errors.New(idioma.Tf("la clave de %s solo se muestra al crearla o regenerarla (se guarda su huella): tramtai acceso token %s --rotar", args[0], args[0]))
			}
			return con(func(cmd *cobra.Command, cli *cliente.Cliente, ctx context.Context, _ string, args []string) error {
				valor, err := cli.RotarTokenEnArchivo(ctx, args[0])
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), idioma.T("clave nueva de %s (guárdala: no se volverá a mostrar):\n%s\n"), args[0], valor)
				return nil
			})(cmd, args)
		}}
	token.Flags().BoolVar(&rotar, "rotar", false, idioma.T("genera una clave nueva; la anterior deja de valer"))
	var instanciaPresupuesto string
	reiniciar := &cobra.Command{Use: "reiniciar-presupuesto [nombre]", Args: cobra.MaximumNArgs(1),
		Short: idioma.T("Pone a cero el consumo del día y del mes de una API o, sin nombre, del espacio"),
		RunE: func(cmd *cobra.Command, args []string) error {
			nombre := ""
			if len(args) == 1 {
				nombre = args[0]
			}
			return reiniciarPresupuesto(cmd, instanciaPresupuesto, nombre)
		}}
	reiniciar.Flags().StringVar(&instanciaPresupuesto, "instancia", "", idioma.T("espacio cuyo presupuesto se reinicia (el global por defecto)"))
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
			return fmt.Errorf(idioma.T("no existe la API %s en el servicio"), nombre)
		}
		instancia = e.Puntos[i].Instancia
	}
	if err := c.ReiniciarPresupuesto(ctx, instancia, nombre); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), idioma.T("consumo reiniciado"))
	return nil
}
