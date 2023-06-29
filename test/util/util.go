package util

import "fmt"

type Monster struct{ O int }

func (m *Monster) Add() (string, *string) {

	go Print(m.O)

	go func() {
		Print(m.O)
	}()

	return Test(m.O), nil
}

func Test(d int) string {
	a := retFunc(d)
	return a()
}

func retFunc(a int) func() string {
	if a == 0 {
		return func() string {
			return "yes"
		}
	}
	if a == 1 {
		return F
	}
	return t
}

var F func() string = func() string {
	return "no"
}

func t() (s string) {
	s = "beside"
	return
}

func Print(d int) {
	a := retFunc(d)
	fmt.Println(a())
}

func A() (x, y string) {
	return a()
}

func a() (string, string) {
	return "", ""
}
