package main

import (
	"fmt"
	"unicode/utf8"
) //printing
//String counting

func main() {
	var intNum int = 32767
	intNum = intNum + 1
	fmt.Println(intNum)

	var Sentence string = "What's up"
	fmt.Println(utf8.RuneCountInString(Sentence))
}
