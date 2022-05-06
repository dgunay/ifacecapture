// Example program that erroneously captures the outer variable when it likely
// intends to use the parameter interface.

package main

type MyInterface interface {
	Do()
}

type MyImpl struct{}

func (m *MyImpl) Do() {}

func doThing(callback func(tx MyInterface)) {
	myImpl := MyImpl{}
	callback(&myImpl)
}

func main() {
	outer := MyImpl{}
	doThing(func(inner MyInterface) {
		outer.Do() // should be inner.Do()
		inner.Do()
	})
}
