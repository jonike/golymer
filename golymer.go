package golymer

import (
	"errors"
	"reflect"

	"github.com/gopherjs/gopherjs/js"
)

//testConstructorFunction tests that it is a function with no attributes and one pointer result
func testConstructorFunction(f interface{}) error {
	if reflect.ValueOf(f).Kind() != reflect.Func {
		return errors.New("Define Error: provided f parameter is not a function (it must be func()*YourElemType)")
	}
	if reflect.TypeOf(f).NumOut() != 1 {
		return errors.New("Define Error: provided function doesn't have one result value (it must be func()*YourElemType)")
	}
	if reflect.TypeOf(f).Out(0).Kind() != reflect.Ptr {
		return errors.New("Define Error: provided function doesn't return an pointer (it must be func()*YourElemType)")
	}
	if elemStruct, ok := reflect.TypeOf(f).Out(0).Elem().FieldByName("Element"); !ok || elemStruct.Type.Name() != "Element" {
		return errors.New("Define Error: provided function doesn't return an struct that has embedded golymer.Element struct (it must be func()*YourElemType)")
	}
	elementName := camelCaseToKebab(reflect.TypeOf(f).Out(0).Elem().Name())
	if !js.InternalObject(elementName).Call("includes", "-").Bool() {
		return errors.New("Define Error: name of the struct type MUST have two words in camel case eg. MyElement will be converted to tag name my-element (it must be func()*YourElemType)")
	}
	return nil
}

//getStructFields returns fields of the provided struct
func getStructFields(customElementType reflect.Type) (customElementFields []reflect.StructField) {
	for i := 0; i < customElementType.NumField(); i++ {
		field := customElementType.Field(i)
		customElementFields = append(customElementFields, field)
	}
	return
}

//setPrototypeCallbacks sets callbacks of CustomElements v1 (connectedCallback, disconnectedCallback, attributeChangedCallback and adoptedCallback)
func setPrototypeCallbacks(prototype *js.Object) {
	prototype.Set("connectedCallback", js.MakeFunc(func(this *js.Object, arguments []*js.Object) interface{} {
		this.Get("__internal_object__").Interface().(CustomElement).ConnectedCallback()
		return nil
	}))
	prototype.Set("disconnectedCallback", js.MakeFunc(func(this *js.Object, arguments []*js.Object) interface{} {
		this.Get("__internal_object__").Interface().(CustomElement).DisconnectedCallback()
		return nil
	}))
	prototype.Set("attributeChangedCallback", js.MakeFunc(func(this *js.Object, arguments []*js.Object) interface{} {
		this.Get("__internal_object__").Interface().(CustomElement).AttributeChangedCallback(
			arguments[0].String(),
			arguments[1].String(),
			arguments[2].String(),
			arguments[3].String(),
		)
		return nil
	}))
	prototype.Set("adoptedCallback", js.MakeFunc(func(this *js.Object, arguments []*js.Object) interface{} {
		this.Get("__internal_object__").Interface().(CustomElement).AdoptedCallback(
			arguments[0].Interface(),
			arguments[1].Interface(),
		)
		return nil
	}))
}

//Define registers an new custom element
//takes the constructor of the element func()*YourElemType
//element is registered under the name converted from your element type (YourElemType -> your-elem-type)
func Define(f interface{}) error {
	if err := testConstructorFunction(f); err != nil {
		return err
	}

	customElementType := reflect.TypeOf(f).Out(0).Elem()
	customElementFields := getStructFields(customElementType)

	element := js.MakeFunc(func(this *js.Object, arguments []*js.Object) interface{} {
		instance := js.Global.Get("Reflect").Call(
			"construct",
			js.Global.Get("HTMLElement"),
			make([]interface{}, 0),
			js.Global.Get(customElementType.Name()),
		)
		customObject := reflect.ValueOf(f).Call(nil)[0]
		customObjectProxy := newProxy(customObject, []string{})
		instance.Set("__internal_object__", customObjectProxy)
		instance.Set("$var", customObjectProxy)
		customObject.Elem().FieldByName("Element").FieldByName("Object").Set(reflect.ValueOf(instance))
		customObject.Elem().FieldByName("Element").FieldByName("ObjValue").Set(reflect.ValueOf(customObject))
		return instance
	})

	js.Global.Set(customElementType.Name(), element)
	prototype := element.Get("prototype")
	js.Global.Get("Object").Call("setPrototypeOf", prototype, js.Global.Get("HTMLElement").Get("prototype"))
	js.Global.Get("Object").Call("setPrototypeOf", element, js.Global.Get("HTMLElement"))

	//getters and setters of the customElement
	for _, field := range customElementFields {
		field := field
		gs := map[string]interface{}{
			"get": js.MakeFunc(func(this *js.Object, arguments []*js.Object) interface{} {
				return this.Get("__internal_object__").Get(field.Name)
			}),
			"set": js.MakeFunc(func(this *js.Object, arguments []*js.Object) interface{} {
				//if the field is exported than the element attribute is also set
				if field.PkgPath == "" {
					setNodeAttribute(field, this, arguments[0])
				} else {
					this.Get("__internal_object__").Set(field.Name, arguments[0])
				}
				return arguments[0]
			}),
		}
		js.Global.Get("Object").Call("defineProperty", prototype, field.Name, gs)
	}

	//observedAttributes getter
	js.Global.Get("Object").Call("defineProperty", element, "observedAttributes", map[string]interface{}{
		"get": js.MakeFunc(func(this *js.Object, arguments []*js.Object) interface{} {
			var observedAttributes []string
			for _, field := range customElementFields {
				//if it's an exported attribute, add it to observedAttributes
				if field.PkgPath != "" {
					continue
				}
				observedAttributes = append(observedAttributes, camelCaseToKebab(field.Name))
			}
			return observedAttributes
		}),
	})

	setPrototypeCallbacks(prototype)

	js.Global.Get("customElements").Call("define", camelCaseToKebab(customElementType.Name()), element)
	return nil
}

//MustDefine registers an new custom element
//takes the constructor of the element func()*YourElemType
//element is registered under the name converted from your element type (YourElemType -> your-elem-type)
//if an error occures it panics
func MustDefine(f interface{}) {
	if err := Define(f); err != nil {
		panic(err)
	}
}

//CreateElement creates a new instance of an element that can be type asserted to custom element
func CreateElement(elementName string) interface{} {
	return js.Global.Get("document").Call("createElement", elementName).Interface()
}
