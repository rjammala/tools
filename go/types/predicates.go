// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements commonly used type predicates.

package types

import "go/ast"

func isNamed(typ Type) bool {
	if _, ok := typ.(*Basic); ok {
		return ok
	}
	_, ok := typ.(*Named)
	return ok
}

func isBoolean(typ Type) bool {
	t, ok := typ.Underlying().(*Basic)
	return ok && t.info&IsBoolean != 0
}

func isInteger(typ Type) bool {
	t, ok := typ.Underlying().(*Basic)
	return ok && t.info&IsInteger != 0
}

func isUnsigned(typ Type) bool {
	t, ok := typ.Underlying().(*Basic)
	return ok && t.info&IsUnsigned != 0
}

func isFloat(typ Type) bool {
	t, ok := typ.Underlying().(*Basic)
	return ok && t.info&IsFloat != 0
}

func isComplex(typ Type) bool {
	t, ok := typ.Underlying().(*Basic)
	return ok && t.info&IsComplex != 0
}

func isNumeric(typ Type) bool {
	t, ok := typ.Underlying().(*Basic)
	return ok && t.info&IsNumeric != 0
}

func isString(typ Type) bool {
	t, ok := typ.Underlying().(*Basic)
	return ok && t.info&IsString != 0
}

func isUntyped(typ Type) bool {
	t, ok := typ.Underlying().(*Basic)
	return ok && t.info&IsUntyped != 0
}

func isOrdered(typ Type) bool {
	t, ok := typ.Underlying().(*Basic)
	return ok && t.info&IsOrdered != 0
}

func isConstType(typ Type) bool {
	t, ok := typ.Underlying().(*Basic)
	return ok && t.info&IsConstType != 0
}

func isComparable(typ Type) bool {
	switch t := typ.Underlying().(type) {
	case *Basic:
		return t.kind != Invalid && t.kind != UntypedNil
	case *Pointer, *Interface, *Chan:
		// assumes types are equal for pointers and channels
		return true
	case *Struct:
		for _, f := range t.fields {
			if !isComparable(f.typ) {
				return false
			}
		}
		return true
	case *Array:
		return isComparable(t.elt)
	}
	return false
}

func hasNil(typ Type) bool {
	switch typ.Underlying().(type) {
	case *Slice, *Pointer, *Signature, *Interface, *Map, *Chan:
		return true
	}
	return false
}

// IsIdentical returns true if x and y are identical.
func IsIdentical(x, y Type) bool {
	if x == y {
		return true
	}

	switch x := x.(type) {
	case *Basic:
		// Basic types are singletons except for the rune and byte
		// aliases, thus we cannot solely rely on the x == y check
		// above.
		if y, ok := y.(*Basic); ok {
			return x.kind == y.kind
		}

	case *Array:
		// Two array types are identical if they have identical element types
		// and the same array length.
		if y, ok := y.(*Array); ok {
			return x.len == y.len && IsIdentical(x.elt, y.elt)
		}

	case *Slice:
		// Two slice types are identical if they have identical element types.
		if y, ok := y.(*Slice); ok {
			return IsIdentical(x.elt, y.elt)
		}

	case *Struct:
		// Two struct types are identical if they have the same sequence of fields,
		// and if corresponding fields have the same names, and identical types,
		// and identical tags. Two anonymous fields are considered to have the same
		// name. Lower-case field names from different packages are always different.
		if y, ok := y.(*Struct); ok {
			if x.NumFields() == y.NumFields() {
				for i, f := range x.fields {
					g := y.fields[i]
					if f.anonymous != g.anonymous ||
						x.Tag(i) != y.Tag(i) ||
						!f.isMatch(g.pkg, g.name) ||
						!IsIdentical(f.typ, g.typ) {
						return false
					}
				}
				return true
			}
		}

	case *Pointer:
		// Two pointer types are identical if they have identical base types.
		if y, ok := y.(*Pointer); ok {
			return IsIdentical(x.base, y.base)
		}

	case *Signature:
		// Two function types are identical if they have the same number of parameters
		// and result values, corresponding parameter and result types are identical,
		// and either both functions are variadic or neither is. Parameter and result
		// names are not required to match.
		if y, ok := y.(*Signature); ok {
			return x.isVariadic == y.isVariadic &&
				identicalTypes(x.params, y.params) &&
				identicalTypes(x.results, y.results)
		}

	case *Interface:
		// Two interface types are identical if they have the same set of methods with
		// the same names and identical function types. Lower-case method names from
		// different packages are always different. The order of the methods is irrelevant.
		if y, ok := y.(*Interface); ok {
			return identicalMethods(x.methods, y.methods) // methods are sorted
		}

	case *Map:
		// Two map types are identical if they have identical key and value types.
		if y, ok := y.(*Map); ok {
			return IsIdentical(x.key, y.key) && IsIdentical(x.elt, y.elt)
		}

	case *Chan:
		// Two channel types are identical if they have identical value types
		// and the same direction.
		if y, ok := y.(*Chan); ok {
			return x.dir == y.dir && IsIdentical(x.elt, y.elt)
		}

	case *Named:
		// Two named types are identical if their type names originate
		// in the same type declaration.
		if y, ok := y.(*Named); ok {
			return x.obj == y.obj
		}
	}

	return false
}

// identicalTypes returns true if both lists a and b have the
// same length and corresponding objects have identical types.
func identicalTypes(a, b *Tuple) bool {
	if a.Len() != b.Len() {
		return false
	}
	if a != nil {
		for i, x := range a.vars {
			y := b.vars[i]
			if !IsIdentical(x.typ, y.typ) {
				return false
			}
		}
	}
	return true
}

// qname computes the "qualified name" of a function.
// TODO(gri) This is similar in functionality to Field.isMatch.
//           Try to consolidate.
func qname(f *Func) string {
	if ast.IsExported(f.name) {
		return f.name
	}
	if f.pkg == nil {
		panic("unexported function without package information")
	}
	return f.pkg.path + "." + f.name
}

// identicalMethods returns true if both slices a and b have the
// same length and corresponding entries have identical types.
// TODO(gri) make this more efficient (e.g., sort them on completion)
func identicalMethods(a, b []*Func) bool {
	if len(a) != len(b) {
		return false
	}

	m := make(map[string]*Func)
	for _, x := range a {
		k := qname(x)
		assert(m[k] == nil) // method list must not have duplicate entries
		m[k] = x
	}

	for _, y := range b {
		k := qname(y)
		if x := m[k]; x == nil || !IsIdentical(x.typ, y.typ) {
			return false
		}
	}

	return true
}

// defaultType returns the default "typed" type for an "untyped" type;
// it returns the incoming type for all other types. If there is no
// corresponding untyped type, the result is Typ[Invalid].
//
func defaultType(typ Type) Type {
	if t, ok := typ.(*Basic); ok {
		k := Invalid
		switch t.kind {
		// case UntypedNil:
		//      There is no default type for nil. For a good error message,
		//      catch this case before calling this function.
		case UntypedBool:
			k = Bool
		case UntypedInt:
			k = Int
		case UntypedRune:
			k = Rune
		case UntypedFloat:
			k = Float64
		case UntypedComplex:
			k = Complex128
		case UntypedString:
			k = String
		}
		typ = Typ[k]
	}
	return typ
}

// missingMethod returns (nil, false) if typ implements T, otherwise
// it returns the first missing method required by T and whether it
// is missing or simply has the wrong type.
// TODO(gri) make method of Type and/or stand-alone predicate.
//
func missingMethod(typ Type, T *Interface) (method *Func, wrongType bool) {
	// TODO(gri): this needs to correctly compare method names (taking package into account)
	// TODO(gri): distinguish pointer and non-pointer receivers
	// an interface type implements T if it has no methods with conflicting signatures
	// Note: This is stronger than the current spec. Should the spec require this?
	if T.IsEmpty() {
		return
	}
	// T.methods.NumEntries() > 0

	if ityp, _ := typ.Underlying().(*Interface); ityp != nil {
		for _, m := range T.methods {
			res := lookupField(ityp, m.pkg, m.name) // TODO(gri) no need to go via lookupField
			if res.mode != invalid && !IsIdentical(res.obj.Type(), m.typ) {
				return m, true
			}
		}
		return
	}

	// a concrete type implements T if it implements all methods of T.
	for _, m := range T.methods {
		res := lookupField(typ, m.pkg, m.name)
		if res.mode == invalid {
			return m, false
		}
		if !IsIdentical(res.obj.Type(), m.typ) {
			return m, true
		}
	}
	return
}