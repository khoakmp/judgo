package main

import (
	"fmt"
	"reflect"
)

type Person struct {
	name string
}
type Student struct {
	class string
	Person
}

func main() {
	var st *Student
	fmt.Println(reflect.TypeOf(st).Size())
}
