// Command migration creates paired timestamped SQL files. Run from backend/.
package main

import (
	"flag"
	"fmt"
	"github.com/yerassyldanay/xchats/backend/migrations"
	"os"
	"time"
)

func main() {
	dir := flag.String("dir", "migrations", "migration root containing sqlite/ and postgres/")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: migration [-dir migrations] descriptive_name")
		os.Exit(2)
	}
	id, err := migrations.Create(*dir, flag.Arg(0), time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Created", id, "for sqlite and postgres")
}
