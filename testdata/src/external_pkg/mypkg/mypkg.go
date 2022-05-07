package mypkg

type MyInterface interface {
	Do()
}

type MyImpl struct{}

var _ MyInterface = (*MyImpl)(nil)

func (m *MyImpl) Do() {}
