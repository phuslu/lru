// Copyright 2024 The Cockroach Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file introspects into Go runtime internals. In order to prevent
// accidental breakage when a new version of Go is released we require manual
// bumping of the go versions supported by adjusting the build tags below. The
// way go version tags work the tag for goX.Y will be declared for every
// subsequent release. So go1.18 will be defined for go1.21, go1.22, etc. The
// build tag "go1.18 && !go1.23" defines the range [go1.18, go1.23) (inclusive
// on go1.18, exclusive on go1.23).

//go:build go1.18

package lru

import (
	"unsafe"
)

//go:linkname fastrand64 runtime.fastrand64
func fastrand64() uint64

// getRuntimeHasher peeks inside the internals of map[K]struct{} and extracts
// the function the runtime generated for hashing type K. This is a bit hacky,
// but we can't use hash/maphash as that hashes only bytes and strings. While
// we could use unsafe.{Slice,String} to pass in arbitrary structs we can't
// pass in arbitrary types and have the hash function sometimes hash the type
// memory and sometimes hash underlying.
//
// NOTE(peter): I did try using reflection on the type K to specialize a hash
// function depending on the type's Kind, but that was measurably slower than
// for integer types. This hackiness is quite localized. If it breaks in a
// future Go version we can either repair it or go the reflection route.
//
// https://github.com/dolthub/maphash provided the inspiration and general
// implementation technique.
func getRuntimeHasher[K comparable]() func(key unsafe.Pointer, seed uintptr) uintptr {
	a := any((map[K]struct{})(nil))
	return (*rtEface)(unsafe.Pointer(&a)).typ.Hasher
}

// From runtime/runtime2.go:eface
type rtEface struct {
	typ  *rtMapType
	data unsafe.Pointer
}

// From internal/abi/type.go:MapType
type rtMapType struct {
	rtType
	Key    *rtType
	Elem   *rtType
	Bucket *rtType // internal type representing a hash bucket
	// function for hashing keys (ptr to key, seed) -> hash
	Hasher     func(unsafe.Pointer, uintptr) uintptr
	KeySize    uint8  // size of key slot
	ValueSize  uint8  // size of elem slot
	BucketSize uint16 // size of bucket
	Flags      uint32
}

type rtTFlag uint8
type rtNameOff int32
type rtTypeOff int32

// From internal/abi/type.go:Type
type rtType struct {
	Size_       uintptr
	PtrBytes    uintptr // number of (prefix) bytes in the type that can contain pointers
	Hash        uint32  // hash of type; avoids computation in hash tables
	TFlag       rtTFlag // extra type information flags
	Align_      uint8   // alignment of variable with this type
	FieldAlign_ uint8   // alignment of struct field with this type
	Kind_       uint8   // enumeration for C
	// function for comparing objects of this type
	// (ptr to object A, ptr to object B) -> ==?
	Equal func(unsafe.Pointer, unsafe.Pointer) bool
	// GCData stores the GC type data for the garbage collector.
	// If the KindGCProg bit is set in kind, GCData is a GC program.
	// Otherwise it is a ptrmask bitmap. See mbitmap.go for details.
	GCData    *byte
	Str       rtNameOff // string form
	PtrToThis rtTypeOff // type for pointer to this type, may be zero
}

// noescape hides a pointer from escape analysis.  noescape is
// the identity function but escape analysis doesn't think the
// output depends on the input.  noescape is inlined and currently
// compiles down to zero instructions.
// USE CAREFULLY!
//
//go:nosplit
//go:nocheckptr
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}

func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
