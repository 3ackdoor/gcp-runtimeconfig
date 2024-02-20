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

var (
	prj = os.Getenv("PROJECT")
	cfg = os.Getenv("CONFIG")
	vrb = os.Getenv("VARIABLE") // TODO: make this variable to point to the key of yml file on runtimeconfig or make a list of variables instead
)

func main() {
	fmt.Println("starting program...")

	rc := &RuntimeConfig{
		config: make(map[string]string, 1),
	}

	rc.watch()
	defer rc.variable.Close()
	periodicallyPrint(rc)

	var st time.Duration = 120 * time.Second
	fmt.Printf("ending program in %v...\n", st)
	time.Sleep(st)

	os.Exit(0)
}

// periodicallyPrint periodically prints the config value.
func periodicallyPrint(rc *RuntimeConfig) {
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for range ticker.C {
			fmt.Printf("(goroutine)config key: %+v, value: %+v\n", vrb, rc.read(vrb))
		}
	}()
}

type RuntimeConfig struct {
	variable *runtimevar.Variable
	config   map[string]string
	rw       sync.RWMutex
}

// initVariable initializes the variable in the RuntimeConfig.
func (rc *RuntimeConfig) initVariable(ctx context.Context) error {
	v, err := runtimevar.OpenVariable(ctx, "gcpruntimeconfig://projects/"+prj+"/configs/"+cfg+"/variables/"+vrb+"?decoder=string")
	if err != nil {
		return err
	}

	rc.variable = v
	return nil
}

// write writes the value for the given key in the RuntimeConfig.
func (rc *RuntimeConfig) write(key string, value string) {
	rc.rw.Lock()
	rc.config[key] = value
	rc.rw.Unlock()
}

// read returns the value associated with the given key in the RuntimeConfig.
func (rc *RuntimeConfig) read(key string) string {
	rc.rw.RLock()
	defer rc.rw.RUnlock()
	return rc.config[key]
}

// watch Call Watch in a loop from a background goroutine to see all changes,
// including errors.
//
// You can use this for logging, or to trigger behaviors when the
// config changes.
//
// Note that Latest always returns the latest "good" config, so seeing
// an error from Watch doesn't mean that Latest will return one.
//
// Maybe add time.Sleep(3 * time.Second) after finished this function to make sure config is
// loaded or call this function after finished executing variable.Latest(ctx) and store the value in a variable.
func (rc *RuntimeConfig) watch() {

	// runtimevar.OpenVariable creates a *runtimevar.Variable from a URL.
	// The URL Host+Path are used as the GCP Runtime Configurator Variable key;
	// see https://cloud.google.com/deployment-manager/runtime-configurator/
	// for more details.

	if rc.variable == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := rc.initVariable(ctx)
		if err != nil {
			fmt.Printf("Error initializing variable: %v", err)
			return
		}
	}

	go func() {
		for {
			snapshot, err := rc.variable.Watch(context.Background())
			if err == runtimevar.ErrClosed {
				// variable has been closed; exit.
				return
			}
			if err == nil {
				// Casting to a string here because we used StringDecoder.
				rc.write(vrb, snapshot.Value.(string))
			} else {
				fmt.Printf("Error loading config: %v", err)
				// Even though there's been an error loading the config,
				// variable.Latest will continue to return the latest "good" value.
			}
		}
	}()
}
