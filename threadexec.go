package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/pkg/errors"
)

func run(c *exec.Cmd) {
	fmt.Printf("running %q\n", c.Args)
	o, err := c.CombinedOutput()
	if err != nil {
		fmt.Printf("!! %+v: %q\n%s\n", err, c.Args, string(o))
	}
}

func profilePNG(fn string) {
	run(exec.Command("c:/utils/optipng.exe", "-o7", "-zm1-9", fn))
	run(exec.Command("c:/utils/advpng.exe", "-z4", fn))
}

func profileZIP(fn string) {
	run(exec.Command("c:/utils/advzip.exe", "-z4k", "-i40", fn))
}

func thread(wg *sync.WaitGroup, fnch chan string, fnp func(string)) {
	runtime.LockOSThread()
	goroutineBackgroundStart()
	for fn := range fnch {
		fnp(fn)
		wg.Done()
	}
	runtime.Goexit() // kill self to clean up thread state
}

func allsize(f []string) (r int64, err error) {
	var s os.FileInfo
	for k := range f {
		s, err = os.Stat(f[k])
		if err != nil {
			err = errors.Wrapf(err, "unable to stat %q", f[k])
			return
		}
		r += s.Size()
	}
	return
}

func must(m string, e error) {
	if e != nil {
		fmt.Fprintf(os.Stderr, "Failed to %s: %v\n", m, e)
		panic(e)
	}
}

func deglob() ([]string, error) {
	r := make([]string, 0, flag.NArg())
	for _, a := range flag.Args() {
		m, err := filepath.Glob(a)
		if err != nil {
			return nil, errors.Wrapf(err, "possibly invalid glob pattern %q", a)
		}
		if len(m) == 0 {
			return nil, fmt.Errorf("no files found matching %q", a)
		}
		r = append(r, m...)
	}
	return r, nil
}

func main() {
	procs := flag.Int("c", runtime.NumCPU(), "maximum number of concurrent processes")
	fsdelta := flag.Bool("s", true, "calculate file sizes before and after operation")
	resched := flag.Bool("i", true, "run process at idle priority")
	profile := flag.String("p", "png", "profile for thread processing") // this will be better some day

	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Println("Supply list of files for processing")
		return
	}

	fns, err := deglob()
	must("find specified files", err)

	var startSize int64
	if *fsdelta {
		startSize, err = allsize(fns)
		must("calculate starting file sizes", err)
		fmt.Printf("Starting size: %d bytes\n", startSize)
	}

	if *resched {
		processBackgroundStart()
	}

	var wg sync.WaitGroup
	wg.Add(len(fns))
	fnch := make(chan string)
	for j := 0; j < *procs; j++ {
		switch *profile {
		case "png":
			go thread(&wg, fnch, profilePNG)
		case "zip":
			go thread(&wg, fnch, profileZIP)
		}
	}
	for k := range fns {
		fnch <- fns[k]
	}
	wg.Wait()
	close(fnch)

	if *fsdelta {
		endSize, err := allsize(fns)
		must("calculate ending file sizes", err)
		fmt.Printf("Ending size: %d bytes (%.2f%% of %d)\n", endSize, (100*float32(endSize))/float32(startSize), startSize)
	}

}
