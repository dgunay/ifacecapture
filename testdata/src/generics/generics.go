package main

func GenericFunc[T any]() {
}

func UseGenericFunc() {
	GenericFunc[string]()
}
