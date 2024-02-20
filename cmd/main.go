package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"gocloud.dev/runtimevar"
	_ "gocloud.dev/runtimevar/gcpruntimeconfig"
)

type RuntimeConfig struct {
	config map[string]string
	rw     sync.RWMutex
}

var rc *RuntimeConfig = &RuntimeConfig{
	config: make(map[string]string, 1),
}

var (
	prj = os.Getenv("PROJECT")
	cfg = os.Getenv("CONFIG")
	vrb = os.Getenv("VARIABLE") // TODO: make this variable to point to the key of yml file on runtimeconfig or make a list of variables instead
)

func main() {
	fmt.Println("starting program...")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// runtimevar.OpenVariable creates a *runtimevar.Variable from a URL.
	// The URL Host+Path are used as the GCP Runtime Configurator Variable key;
	// see https://cloud.google.com/deployment-manager/runtime-configurator/
	// for more details.

	v, err := runtimevar.OpenVariable(ctx, "gcpruntimeconfig://projects/"+prj+"/configs/"+cfg+"/variables/"+vrb+"?decoder=string")
	if err != nil {
		e := fmt.Errorf("unable to open variable: %v", err)
		fmt.Println(e)
		return
	}
	defer v.Close()

	periodicallyPrint()
	watch(v)

	rc.rw.RLock()
	fmt.Printf("(main)config: %+v\n", rc.config)
	rc.rw.RUnlock()

	var st time.Duration = 10 * time.Second
	fmt.Printf("ending program in %v...\n", st)
	time.Sleep(st)

	os.Exit(0)
}

// periodicallyPrint periodically prints the config value.
func periodicallyPrint() {
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				rc.rw.RLock()
				fmt.Printf("(goroutine)config: %+v\n", rc.config)
				rc.rw.RUnlock()
			}
		}
	}()
}

// watch Call Watch in a loop from a background goroutine to see all changes,
// including errors.
//
// You can use this for logging, or to trigger behaviors when the
// config changes.
//
// Note that Latest always returns the latest "good" config, so seeing
// an error from Watch doesn't mean that Latest will return one.
func watch(v *runtimevar.Variable) {
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				snapshot, err := v.Watch(context.Background())
				if err == runtimevar.ErrClosed {
					// v has been closed; exit.
					return
				}
				if err == nil {
					// Casting to a string here because we used StringDecoder.
					// fmt.Printf("New config: %+v", snapshot.Value.(string))
					rc.rw.Lock()
					rc.config[vrb] = snapshot.Value.(string)
					rc.rw.Unlock()
				} else {
					fmt.Printf("Error loading config: %v", err)
					// Even though there's been an error loading the config,
					// v.Latest will continue to return the latest "good" value.
				}
			}
		}
	}()
}
