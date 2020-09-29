package got

import (
	"reflect"
)

const prefixEachErr = "[got.Each]"

// Each runs each exported method Fn on type Ctx as a subtest of t.
// The iteratee can be a struct Ctx or:
//
//     iteratee(t Testable) (ctx Ctx)
//
// Each Fn will be called as:
//
//      ctx.Fn()
//
// If iteratee is Ctx, Each will try to set its Assertion field to New(Testable) on each test.
// Embedded Fn will be ignored.
func Each(t Testable, iteratee interface{}) (count int) {
	t.Helper()

	itVal := normalizeIteratee(t, iteratee)

	ctxType := itVal.Type().Out(0)

	methods := filterMethods(ctxType)

	for _, method := range methods {
		run := reflect.ValueOf(t).MethodByName("Run")

		checkFnType(t, method)

		run.Call([]reflect.Value{
			reflect.ValueOf(method.Name),
			reflect.MakeFunc(run.Type().In(1), func(args []reflect.Value) []reflect.Value {
				t := args[0].Interface().(Testable)
				t.Helper()

				var res []reflect.Value
				if iteratee != nil {
					res = itVal.Call(args)
				}

				method.Func.Call(res)
				return nil
			}),
		})
	}
	return len(methods)
}

func normalizeIteratee(t Testable, iteratee interface{}) reflect.Value {
	t.Helper()

	if iteratee == nil {
		t.Logf(prefixEachErr + " iteratee shouldn't be nil")
		t.FailNow()
	}

	itVal := reflect.ValueOf(iteratee)
	itType := itVal.Type()

	defer func() {
		if recover() == nil {
			return
		}
		t.Logf(prefixEachErr+" iteratee <%v> should be a struct or <func(got.Testable) Ctx>", itType)
		t.FailNow()
	}()

	switch itType.Kind() {
	case reflect.Func:
		if itType.NumIn() != 1 || itType.NumOut() != 1 {
			panic("")
		}
		_ = reflect.New(itType.In(0).Elem()).Interface().(Testable) // this may panic
		return itVal

	case reflect.Struct:
		fnType := reflect.FuncOf([]reflect.Type{reflect.TypeOf(t)}, []reflect.Type{itType}, false)
		return reflect.MakeFunc(fnType, func(args []reflect.Value) []reflect.Value {
			t := args[0].Interface().(Testable)
			as := reflect.ValueOf(New(t))

			c := reflect.New(itType).Elem()
			c.Set(itVal)
			try(func() {
				c.FieldByName("Assertion").Set(as)
			})

			return []reflect.Value{c}
		})
	}

	panic("")
}

func checkFnType(t Testable, fn reflect.Method) {
	t.Helper()

	if fn.Type.NumIn() != 1 || fn.Type.NumOut() != 0 {
		t.Logf(prefixEachErr+" %s.%s shouldn't have arguments or return values", fn.Type.In(0).String(), fn.Name)
		t.FailNow()
	}
}

func filterMethods(typ reflect.Type) []reflect.Method {
	fields := []reflect.StructField{}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Anonymous {
			fields = append(fields, field)
		}
	}

	methods := []reflect.Method{}

	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)

		for _, field := range fields {
			if m, ok := field.Type.MethodByName(method.Name); ok {
				if methodEq(m, method) {
					goto skip
				}
			}
		}
		methods = append(methods, method)
	skip:
	}

	return methods
}

func methodEq(a, b reflect.Method) bool {
	if a.Type.NumIn() != b.Type.NumIn() {
		return false
	}
	for i := 1; i < a.Type.NumIn(); i++ {
		if a.Type.In(i) != b.Type.In(i) {
			return false
		}
	}

	if a.Type.NumOut() != b.Type.NumOut() {
		return false
	}
	for i := 0; i < a.Type.NumOut(); i++ {
		if a.Type.Out(i) != b.Type.Out(i) {
			return false
		}
	}

	return true
}
