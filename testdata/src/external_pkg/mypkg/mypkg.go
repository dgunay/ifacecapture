package mypkg

type MyInterface interface {
	Do()
}

type MyImpl struct{}

var _ MyInterface = (*MyImpl)(nil)

func (m *MyImpl) Do() {}

func (m *MyImpl) DoThing(callback func(tx MyInterface)) {
	myImpl := MyImpl{}
	callback(&myImpl)
}