package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

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
}

func (o *opcionesPunto) registrar(cmd *cobra.Command) {
	f := cmd.Flags()
	f.IntVar(&o.puerto, "puerto", 0, "puerto propio (0 = ruta /p/<nombre> en el puerto del servicio)")
	f.StringVar(&o.instrucciones, "instrucciones", "", "texto que se antepone a cada solicitud, por ejemplo \"Responde siempre en español\"")
	f.StringVar(&o.grupo, "grupo", "", "grupo para ordenar los puntos y activarlos o desactivarlos juntos")
	f.StringVar(&o.instancia, "instancia", "", "instancia a la que pertenece (global, o la del proyecto con --local)")
	f.StringVar(&o.agente, "agente", "", "agente predeterminado del punto")
	f.StringVar(&o.modelo, "modelo", "", "modelo predeterminado del punto")
	f.StringVar(&o.perfil, "perfil", "", "perfil de modelo predeterminado del punto")
	f.StringSliceVar(&o.contratos, "contratos", nil, "API habilitadas: nativa, openai, anthropic (todas por defecto)")
	f.StringSliceVar(&o.permitidos, "permitidos", nil, "modelos permitidos: agente, agente/modelo o perfil/nombre")
	f.StringSliceVar(&o.alias, "alias", nil, "alias de modelos, por ejemplo gpt-4o-mini=claude/haiku")
	f.BoolVar(&o.forzar, "forzar", false, "ignora el modelo de la solicitud y usa el del punto")
	f.StringVar(&o.modoContexto, "modo-contexto", "", "modo de contexto del punto")
	f.StringVar(&o.formato, "formato", "", "formato de salida del punto")
	f.IntVar(&o.tiempo, "tiempo", 0, "tiempo límite en segundos del punto")
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
				return fmt.Errorf("alias no válido %q: usa nombre=agente/modelo", par)
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
	_, err := p.Normalizar("global", 0)
	return err
}

func comandoAcceso() *cobra.Command {
	grupo := &cobra.Command{Use: "acceso", Short: "Puntos de acceso: API adicionales con su puerto o ruta, token y modelos"}
	var local bool
	grupo.PersistentFlags().BoolVar(&local, "local", false, "usa .tramtai/local.yaml del proyecto actual en lugar de la configuración global")
	archivo := func() (string, nucleo.Origen, error) {
		if !local {
			ruta, err := nucleo.RutaGlobal()
			return ruta, nucleo.OrigenGlobal, err
		}
		raiz := raizProyecto()
		if raiz == "" {
			return "", 0, fmt.Errorf("--local exige estar en un proyecto (.tramtai/config.yaml)")
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
		fmt.Fprintln(cmd.OutOrStdout(), "guardado en "+ruta)
		recargarSiActivo(cmd)
		return nil
	}
	buscar := func(lista []servicio.ConfigPunto, nombre string) (int, error) {
		i := slices.IndexFunc(lista, func(p servicio.ConfigPunto) bool { return p.Nombre == nombre })
		if i < 0 {
			return i, fmt.Errorf("no existe el punto de acceso %s en este archivo", nombre)
		}
		return i, nil
	}

	listar := &cobra.Command{Use: "listar", Short: "Lista los puntos de acceso y su estado",
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
			fmt.Fprintln(cmd.OutOrStdout(), "(servicio inactivo; puntos definidos en "+ruta+")")
			for _, p := range puntos {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-18s puerto %-6d activo %v\n", p.Nombre, p.Puerto, p.EstaActivo())
			}
			return nil
		}}

	var crearOpc, editarOpc opcionesPunto
	crear := &cobra.Command{Use: "crear <nombre>", Short: "Crea un punto de acceso y su token", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := servicio.ConfigPunto{Nombre: args[0]}
			if err := crearOpc.aplicar(cmd, &p); err != nil {
				return err
			}
			if err := editar(cmd, func(lista []servicio.ConfigPunto) ([]servicio.ConfigPunto, error) {
				if _, err := buscar(lista, p.Nombre); err == nil {
					return nil, fmt.Errorf("el punto de acceso %s ya existe: usa tramtai acceso editar", p.Nombre)
				}
				return append(lista, p), nil
			}); err != nil {
				return err
			}
			return mostrarTokenPunto(cmd, p.Nombre, false)
		}}
	crearOpc.registrar(crear)
	editarCmd := &cobra.Command{Use: "editar <nombre>", Short: "Cambia las opciones indicadas de un punto de acceso", Args: cobra.ExactArgs(1),
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
			Short: map[bool]string{true: "Activa", false: "Desactiva"}[valor] + " uno o varios puntos de acceso sin reiniciar el servicio",
			RunE: func(cmd *cobra.Command, args []string) error {
				if (len(args) == 0) == (grupoPuntos == "") {
					return fmt.Errorf("indica los nombres o --grupo, no ambos")
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
							return nil, fmt.Errorf("ningún punto de acceso del grupo %s en este archivo", grupoPuntos)
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
		c.Flags().StringVar(&grupoPuntos, "grupo", "", "todos los puntos de ese grupo")
		return c
	}
	eliminar := &cobra.Command{Use: "eliminar <nombre>", Short: "Elimina un punto de acceso", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return editar(cmd, func(lista []servicio.ConfigPunto) ([]servicio.ConfigPunto, error) {
				i, err := buscar(lista, args[0])
				if err != nil {
					return nil, err
				}
				return slices.Delete(lista, i, i+1), nil
			})
		}}
	var rotar bool
	token := &cobra.Command{Use: "token <nombre>", Short: "Muestra (o rota con --rotar) el token del punto de acceso", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return mostrarTokenPunto(cmd, args[0], rotar) }}
	token.Flags().BoolVar(&rotar, "rotar", false, "genera un token nuevo; el anterior deja de valer")
	grupo.AddCommand(listar, crear, editarCmd, activo("activar", true), activo("desactivar", false), eliminar, token)
	return grupo
}

func mostrarTokenPunto(cmd *cobra.Command, nombre string, rotar bool) error {
	datos, err := dirDatos()
	if err != nil {
		return err
	}
	ruta := servicio.RutaTokenPunto(datos, nombre)
	var valor string
	if rotar {
		valor, err = servicio.RotarToken(ruta)
	} else {
		valor, err = servicio.LeerOCrearToken(ruta)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "token de %s (%s):\n%s\n", nombre, ruta, valor)
	if rotar {
		recargarSiActivo(cmd)
	}
	return nil
}

// recargarSiActivo aplica los cambios en el servicio en marcha; si no lo está, se aplicarán al iniciarlo.
func recargarSiActivo(cmd *cobra.Command) {
	c, err := conectar()
	if err != nil {
		fmt.Fprintln(cmd.OutOrStdout(), "el servicio no está en marcha: el cambio se aplicará al iniciarlo")
		return
	}
	ctx, cancelar := contexto()
	defer cancelar()
	puntos, err := c.Recargar(ctx)
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "no se pudo recargar:", err)
		return
	}
	imprimirPuntos(cmd.OutOrStdout(), puntos)
}
