// Command devtoys 是 DevToys 的桌面入口。
package main

import (
	"log"

	"github.com/Neuz/DevToys/internal/app"
	"github.com/egoist/mygo"
)

func main() {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
