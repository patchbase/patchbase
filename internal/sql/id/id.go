package id

import (
	gonanoid "github.com/matoous/go-nanoid/v2"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
const length = 14

func New(tablePrefix string) string {
	id := gonanoid.MustGenerate(alphabet, length)
	return tablePrefix + "_" + id
}
