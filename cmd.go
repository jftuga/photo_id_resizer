/*
photo_id_resizer
-John Taylor
Nov-26-2020

Resize photo ID images using face recognition technology.
*/

package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/esimov/caire"
)

type result struct {
	path string
	err  error
}

const pgmName = "photo_id_resizer"
const pgmUrl = "https://github.com/jftuga/photo_id_resizer"
const pgmVersion = "1.2.0"
const equalsLine = "=============================================================="

// copy - copy a src file to a dst directory
func copy(src, dst string) (int64, error) {
	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

// needsResizing - return true if source image has height greater than maxHeight
// or image has width greater than maxWidth
func needsResizing(path string, maxHeight, maxWidth int) bool {
	if reader, err := os.Open(path); err == nil {
		defer reader.Close()
		im, _, err := image.DecodeConfig(reader)
		if err != nil {
			log.Printf("needsResizing(): %s: %v\n", path, err)
			return false
		}
		if im.Height > maxHeight+1 {
			return true
		}
		if im.Width > maxWidth+1 {
			return true
		}
	}
	return false
}

// isOlderThan - return true if the given time, t is older than maxAge days
func isOlderThan(maxAge int, t time.Time) bool {
	days := maxAge * -1
	earlier := time.Now().AddDate(0, 0, days)
	return t.Before(earlier)
}

// process - examine a single srcname, resize if necessary
// and then save or copy to dstname
func process(p *caire.Processor, dstname, srcname string) error {
	var src io.Reader
	_, err := os.Stat(srcname)
	if err != nil {
		log.Fatalf("Unable to open source: %v", err)
	}
	if !needsResizing(srcname, p.NewHeight, p.NewWidth) {
		copy(srcname, dstname)
		return nil
	}

	f, err := os.Open(srcname)
	if err != nil {
		log.Fatalf("Unable to open source file: %v", err)
	}
	defer f.Close()
	src = f

	var dst io.Writer
	f, err = os.OpenFile(dstname, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		log.Fatalf("Unable to open output file: %v", err)
	}
	defer f.Close()
	dst = f

	err = p.Process(src, dst)
	if err == nil {
		fmt.Printf("file resized to: %s \n", path.Base(dstname))
		fmt.Println(equalsLine)
	} else {
		log.Printf("\nError rescaling image %s. Reason: %s\n", srcname, err.Error())
		copy(srcname, dstname)
	}

	return err
}

// walkFiles starts a goroutine to walk the directory tree at source and send the
// path of each regular file on the string channel.  It sends the result of the
// walk on the error channel.  If done is closed, walkFiles abandons its work.
func walkFiles(done <-chan struct{}, source string, match, exclude string, maxAge int) (<-chan string, <-chan error) {
	paths := make(chan string)
	errc := make(chan error, 1)

	usingExclude := false
	var excludeMatched *regexp.Regexp
	var err error

	if len(exclude) > 0 {
		usingExclude = true
		excludeMatched, err = regexp.Compile(exclude)
		if err != nil {
			log.Fatalf("Invalid regular expression: %s\n", exclude)
		}
	}

	var includeMatched *regexp.Regexp
	includeMatched, err = regexp.Compile(match)
	if err != nil {
		log.Fatalf("Invalid regular expression: %s\n", match)
	}

	go func() {
		// Close the paths channel after Walk returns.
		defer close(paths)
		// No select needed for this send, since errc is buffered.
		errc <- filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fmt.Println("name: ", info.Name())
			if usingExclude && excludeMatched.Match([]byte(info.Name())) {
				fmt.Printf("    file excluded via reg expr : %v\n", exclude)
				fmt.Println(equalsLine)
				return nil
			}
			if !includeMatched.Match([]byte(info.Name())) {
				fmt.Printf("    file didn't match : %v\n", match)
				fmt.Println(equalsLine)
				return nil
			}
			if !info.Mode().IsRegular() {
				fmt.Println("    file is not regular")
				fmt.Println(equalsLine)
				return nil
			}
			if maxAge > 0 && isOlderThan(maxAge, info.ModTime()) {
				fmt.Printf("    file is too old   : %v\n", info.ModTime())
				fmt.Println(equalsLine)
				return nil
			} else {
				fmt.Printf("    file is new enough: %v\n", info.ModTime())
				fmt.Println(equalsLine)
			}
			select {
			case paths <- path:
			case <-done:
				return errors.New("walk canceled")
			}
			return nil
		})
	}()

	return paths, errc
}

// digester reads path names from paths and sends digests of the corresponding
// files on c until either paths or done is closed.
func digester(done <-chan struct{}, paths <-chan string, dest string, p *caire.Processor, c chan<- result) {
	var err error
	for path := range paths {
		destFile := filepath.Join(dest, filepath.Base(path))
		process(p, destFile, path)

		select {
		case c <- result{path, err}:
		case <-done:
			return
		}
	}
}

// ImageSizeAll reads all the files in the file tree rooted at root and returns a map
func ImageSizeAll(source, match, exclude, dest string, numWorkers, maxAge int, p *caire.Processor) error {
	done := make(chan struct{})
	defer close(done)

	paths, errc := walkFiles(done, source, match, exclude, maxAge)

	// Start a fixed number of goroutines to read and digest files.
	c := make(chan result)
	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			digester(done, paths, dest, p, c)
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(c)
	}()
	// End of pipeline.

	// consume c
	for r := range c {
		r.path += ""
	}

	if err := <-errc; err != nil {
		return err
	}

	return nil
}

// fileExists - return true if given file exists
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// dirExists - return true if given directory exists
func dirExists(dirname string) bool {
	info, err := os.Stat(dirname)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// usgae - output program's usage
func usage() {
	pgmName := os.Args[0]
	if strings.HasPrefix(os.Args[0], "./") {
		pgmName = os.Args[0][2:]
	}
	fmt.Fprintf(os.Stderr, "\n%s: resize photo ID image files\n", pgmName)
	fmt.Fprintf(os.Stderr, "version: %s\n", pgmVersion)
	fmt.Fprintf(os.Stderr, "%s\n\n", pgmUrl)
	flag.PrintDefaults()
}

// main - process command-line arguments, do some error checking
// and then call ImageSizeAll()
func main() {
	argsSource := flag.String("s", "", "source directory")
	argsDestination := flag.String("d", "", "destination directory")
	argsHeight := flag.Int("h", 0, "max image height")
	argsWidth := flag.Int("w", 0, "max image width")
	argsMatch := flag.String("m", "jpg|png", "regular expression to match files. Ex: jpg")
	argsExclude := flag.String("x", "", "regular expression to exclude files, precedes -m")
	argsFace := flag.String("f", "facefinder", "path to 'facefinder' classification file")
	argsWorkers := flag.Int("t", runtime.NumCPU(), "number of files to process concurrently")
	argsMaxAge := flag.Int("a", 0, "skip files older than X number of days. Ex: 0=do not skip any, 7=skip files older than a week")
	flag.Usage = usage
	flag.Parse()

	if len(*argsSource) == 0 || len(*argsDestination) == 0 {
		usage()
		os.Exit(1)
	}

	if !fileExists(*argsFace) {
		log.Fatalf("Classification file not found: %s", *argsFace)
	}

	if !dirExists(*argsSource) {
		log.Fatalf("Source directory does not exist: %s", *argsSource)
	}

	if !dirExists(*argsDestination) {
		err := os.Mkdir(*argsDestination, 0700)
		if err != nil {
			log.Fatalf("Destination directory does not exist: %s ; %s\n", *argsDestination, err)
		}
	}

	if *argsHeight == 0 && *argsWidth == 0 {
		fmt.Fprintf(os.Stderr, "\nYou must provide either a -h and/or -w command-line option.\n")
		os.Exit(1)
	}

	if *argsHeight > 0 && *argsWidth > 0 {
		fmt.Fprintf(os.Stderr, "\nWARNING: Using both -h and -w together may lead to undesirable results!\n\n")
	}

	p := &caire.Processor{
		BlurRadius:     10,
		SobelThreshold: 1,
		NewWidth:       *argsWidth,
		NewHeight:      *argsHeight,
		Percentage:     false,
		Square:         false,
		Debug:          false,
		Scale:          true,
		FaceDetect:     true,
		FaceAngle:      0,
		Classifier:     *argsFace,
	}

	ImageSizeAll(*argsSource, *argsMatch, *argsExclude, *argsDestination, *argsWorkers, *argsMaxAge, p)
}
