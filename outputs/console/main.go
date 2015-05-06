package console

import (
	"fmt"
)

func Handler(messageOutgoingQueue <-chan []string) {
	for m := range messageOutgoingQueue {
		for _, l := range m {
			fmt.Println(l)
		}
	}
}