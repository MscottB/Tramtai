package carpetas

import "unsafe"

// unsafePointer convierte el comienzo del SID de una ACE en un puntero, solo para las pruebas.
func unsafePointer(p *uint32) unsafe.Pointer { return unsafe.Pointer(p) }
