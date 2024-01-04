package lib

import (
	"fmt"
	"math/rand"
	"time"
)

func GeneRandomID() string {
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)

	i := random.Intn(1e8)
	return fmt.Sprintf("%08d", i)
}
