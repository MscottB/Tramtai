// Package configuracion edita los archivos de configuración YAML (config.yaml y local.yaml) conservando el resto del
// archivo y sus comentarios: cada cambio se valida con el esquema de nucleo antes de escribirse de forma atómica.
package configuracion

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"

	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// LeerPuntos decodifica la lista puntos_acceso; un campo desconocido o mal tipado es un error.
func LeerPuntos(lista []any) ([]contrato.ConfigPunto, error) {
	if len(lista) == 0 {
		return nil, nil
	}
	datos, err := json.Marshal(lista)
	if err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.T("puntos_acceso no válido"))
	}
	d := json.NewDecoder(bytes.NewReader(datos))
	d.DisallowUnknownFields()
	var puntos []contrato.ConfigPunto
	if err := d.Decode(&puntos); err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, "puntos_acceso: "+err.Error())
	}
	return puntos, nil
}

// EditarPuntos modifica la lista puntos_acceso de un archivo YAML conservando el resto del archivo y sus
// comentarios. Si el resultado no es una configuración válida, deja el archivo como estaba.
func EditarPuntos(ruta string, origen nucleo.Origen, editar func([]contrato.ConfigPunto) ([]contrato.ConfigPunto, error)) error {
	return editarYAML(ruta, origen, func(raiz *yaml.Node) error {
		indice := -1
		var lista []any
		for i := 0; i+1 < len(raiz.Content); i += 2 {
			if raiz.Content[i].Value == "puntos_acceso" {
				indice = i + 1
				if err := raiz.Content[indice].Decode(&lista); err != nil {
					return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("%s: puntos_acceso debe ser una lista", ruta))
				}
				break
			}
		}
		puntos, err := LeerPuntos(lista)
		if err != nil {
			return err
		}
		if puntos, err = editar(puntos); err != nil {
			return err
		}
		var valor yaml.Node
		if err := valor.Encode(puntos); err != nil {
			return err
		}
		if len(puntos) == 0 {
			valor = yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		}
		if indice >= 0 {
			raiz.Content[indice] = &valor
		} else {
			raiz.Content = append(raiz.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "puntos_acceso"}, &valor)
		}
		return nil
	})
}

// FijarValor escribe una clave con puntos (adaptadores.claude.autenticacion) en un archivo YAML, creando los
// mapas que falten y conservando el resto del archivo y sus comentarios.
func FijarValor(ruta string, origen nucleo.Origen, clave string, valor any) error {
	return editarYAML(ruta, origen, func(nodo *yaml.Node) error {
		partes := strings.Split(clave, ".")
		for i, parte := range partes {
			if nodo.Kind != yaml.MappingNode {
				return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("%s: %s debe ser un mapa", ruta, strings.Join(partes[:i], ".")))
			}
			var hijo *yaml.Node
			for j := 0; j+1 < len(nodo.Content); j += 2 {
				if nodo.Content[j].Value == parte {
					hijo = nodo.Content[j+1]
					break
				}
			}
			if hijo == nil {
				hijo = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
				nodo.Content = append(nodo.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: parte}, hijo)
			}
			if i == len(partes)-1 {
				var nuevo yaml.Node
				if err := nuevo.Encode(valor); err != nil {
					return err
				}
				*hijo = nuevo
			}
			nodo = hijo
		}
		return nil
	})
}

// QuitarValor borra una clave con puntos de un archivo YAML (si está), conservando el resto del archivo y sus
// comentarios: «Volver al de Global» quita el valor propio del proyecto.
func QuitarValor(ruta string, origen nucleo.Origen, clave string) error {
	return editarYAML(ruta, origen, func(nodo *yaml.Node) error {
		partes := strings.Split(clave, ".")
		for i, parte := range partes {
			if nodo.Kind != yaml.MappingNode {
				return nil
			}
			j := 0
			for j+1 < len(nodo.Content) && nodo.Content[j].Value != parte {
				j += 2
			}
			if j+1 >= len(nodo.Content) {
				return nil // falta este tramo: la clave no está
			}
			if i == len(partes)-1 {
				nodo.Content = append(nodo.Content[:j], nodo.Content[j+2:]...)
				return nil
			}
			nodo = nodo.Content[j+1]
		}
		return nil
	})
}

// muYAML serializa leer → editar → guardar de los archivos de configuración dentro del proceso: dos ediciones a la vez
// (dos pestañas de la Web UI, un ajuste y una API) no se pisan.
var muYAML sync.Mutex

// editarYAML aplica editar a la raíz de un archivo YAML y lo guarda; si el resultado no es una configuración
// válida, no toca el archivo. El resultado se valida antes de escribir y se escribe de forma atómica: quien lo lee a
// la vez nunca ve un YAML a medias.
func editarYAML(ruta string, origen nucleo.Origen, editar func(raiz *yaml.Node) error) error {
	muYAML.Lock()
	defer muYAML.Unlock()
	original, err := os.ReadFile(ruta)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var doc yaml.Node
	if len(bytes.TrimSpace(original)) > 0 {
		if err := yaml.Unmarshal(original, &doc); err != nil {
			return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("%s: YAML no válido: %s", ruta, err.Error()))
		}
	}
	if doc.Kind == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	raiz := doc.Content[0]
	if raiz.Kind != yaml.MappingNode {
		return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("%s: la raíz debe ser un mapa", ruta))
	}
	if err := editar(raiz); err != nil {
		return err
	}
	salida, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	if err := nucleo.ValidarCambio(original, salida, ruta, origen); err != nil {
		return err
	}
	return EscribirAtomico(ruta, salida, 0o600)
}
