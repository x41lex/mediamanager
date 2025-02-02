package filedb

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
)

func isValidFile(f *File) error {
	if f.id == 0 {
		// Invalid ID, SQL ids start at 1.
		return errors.New("invalid file id")
	}
	return nil
}

// Deprectaed: Use getFileInfo
func hashFile(file *os.File) (string, error) {
	hsh := sha256.New()
	_, err := io.Copy(hsh, file)
	if err != nil {
		return "", fmt.Errorf("failed to create hash: %v", err)
	}
	return fmt.Sprintf("%x", hsh.Sum(nil)), nil
}

func getFileInfo(file *os.File) (hash []byte, size int64, err error) {
	st, err := file.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to stat file: %v", err)
	}
	hsh := sha256.New()
	_, err = io.Copy(hsh, file)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create hash: %v", err)
	}
	return hsh.Sum(nil), st.Size(), nil
}

// Deprecated: Use addInfoToFilesRoutine
//
// errorCh, only the final error will be returned by the parent function
func fileInfoRoutine(recv <-chan *File, wg *sync.WaitGroup, prog *atomic.Int64) {
	defer wg.Done()
	for f := range recv {
		// Open file
		file, err := os.Open(f.GetPath())
		if err != nil {
			slog.Warn("Failed to open file", "Path", f.GetPath(), "Error", err.Error())
			prog.Add(1)
			continue
		}
		// Get file size
		st, err := file.Stat()
		if err != nil {
			slog.Error("Failed to stat opened file", "Path", f.GetPath(), "Error", err.Error())
			prog.Add(1)
			continue
		}
		// Set size
		f.SetSize(st.Size())
		// Hash the file
		h, err := hashFile(file)
		file.Close()
		if err != nil {
			slog.Warn("Failed to hash file", "Path", f.GetPath())
			prog.Add(1)
			continue
		}
		prog.Add(1)
		f.hash = h
	}
}

// Deprecated: Use AddInfoToFiles with opts.ProgressBarWriter as os.Stdout
//
// Add hashes & size to all given files, if a error occurs some files may have hashes, but some files do not.
func AddFileInfoWithProgressBar(goroutines int, f ...*File) error {
	pg := atomic.Int64{}
	fmt.Printf("Adding info to %d files\n", len(f))
	wg := sync.WaitGroup{}
	fileCh := make(chan *File, goroutines*3)
	wg.Add(goroutines)
	for range goroutines {
		go fileInfoRoutine(fileCh, &wg, &pg)
	}
	showProgressBar := true
	go func() {
		// Hide cursor
		fmt.Printf("\x1b[?25l")
		// Show cursor
		defer fmt.Printf("\x1b[?25hDone!\n")
		last := int64(0xfffffffffffffff)
		for showProgressBar {
			n := pg.Load()
			if last != n {
				// Clear line, then clear everything after the line
				fmt.Printf("\r* %d of %d files have info\x1b[0K", n, len(f))
				last = n
			}
		}
	}()
	for _, v := range f {
		fileCh <- v
	}
	// All previously sent files will be got first.
	close(fileCh)
	wg.Wait()
	showProgressBar = false
	for _, v := range f {
		if v.hash == "" || v.GetSize() == 0 {
			return errors.New("not all files were hashed")
		}
	}
	return nil
}

// Deprecated: Use AddInfoToFiles
//
// Add hashes & size to all given files, if a error occurs some files may have hashes, but some files do not.
func AddFileInfo(goroutines int, f ...*File) error {
	pg := atomic.Int64{}
	wg := sync.WaitGroup{}
	fileCh := make(chan *File, goroutines*3)
	wg.Add(goroutines)
	for range goroutines {
		go fileInfoRoutine(fileCh, &wg, &pg)
	}
	for _, v := range f {
		fileCh <- v
	}
	// All previously sent files will be got first.
	close(fileCh)
	wg.Wait()
	for _, v := range f {
		if v.hash == "" || v.GetSize() == 0 {
			return errors.New("not all files were hashed")
		}
	}
	return nil
}

func addInfoHash(f *File) error {
	file, err := os.Open(f.GetPath())
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()
	h, err := hashFile(file)
	if err != nil {
		return fmt.Errorf("failed to hash: %v", err)
	}
	f.hash = h
	slog.Debug("Goroutine adding hash to file", "Id", f.GetId(), "Path", f.GetPath(), "Hash", h)
	return nil
}

func addInfoSize(f *File) error {
	st, err := os.Stat(f.GetPath())
	if err != nil {
		return fmt.Errorf("failed to stat file: %v", err)
	}
	f.SetSize(st.Size())
	slog.Debug("Goroutine adding size to file", "Id", f.GetId(), "Path", f.GetPath(), "Size", st.Size())
	return nil
}

func addInfoToFilesRoutine(recv <-chan *File, wg *sync.WaitGroup, prog *atomic.Int64, ctx context.Context, withHash bool, withSize bool) {
	if !withHash && !withSize {
		panic("FileDb: addInfoToFilesRoutine: withHash & withSize are both false.")
	}
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			// Abort (Context closed)
			return
		case f := <-recv:
			if f == nil {
				// Channel closed.
				return
			}
			if withHash {
				err := addInfoHash(f)
				if err != nil {
					slog.Warn("Failed to add hash to file", "Path", f.GetPath(), "Error", err.Error())
				}
			}
			if withSize {
				err := addInfoSize(f)
				if err != nil {
					slog.Warn("Failed to add size to file", "Path", f.GetPath(), "Error", err.Error())
				}
			}
			prog.Add(1)
		}
	}
}

type AddInfoOpts struct {
	DontAddHash  bool            // Don't add SHA-256 hashes to files. Default: false
	DontAddSize  bool            // Don't add file sizes to files. Default: false
	Goroutines   int             // Goroutines to use for file updating. Default: 100, this is very IO bound, going much higher then this doesn't change much.
	Context      context.Context // Context to wait on. Default: context.Background
	ProgressChan chan<- int64    // The current completed number will be sent every time the value changes, -1 will be sent when done.
	// Deprecated: Use OnCompleteCallback
	//
	//Where to write the progress bar to, if nil no progress bar will be written. Default: nil
	ProgressBarWriter io.Writer
}

// Add/Update info to files
//
// If opts is nil the default options will be used.
func AddInfoToFiles(opts *AddInfoOpts, files ...*File) error {
	if len(files) == 0 {
		return errors.New("0 files passed to AddInfoToFiles")
	}
	// Verify options
	if opts == nil {
		opts = &AddInfoOpts{}
	}
	if opts.Goroutines == 0 {
		opts.Goroutines = 100
	}
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	if opts.DontAddHash && opts.DontAddSize {
		// No operation?
		return errors.New("hash or size must be added")
	}
	// Setup context
	ctx, cancel := context.WithCancel(opts.Context)
	// Setup goroutines
	pg := atomic.Int64{}
	wg := sync.WaitGroup{}
	fileCh := make(chan *File, opts.Goroutines*3)
	wg.Add(opts.Goroutines)
	slog.Debug("Starting goroutines for adding file info", "Options", opts, "Files", len(files))
	// Run them
	for range opts.Goroutines {
		go addInfoToFilesRoutine(fileCh, &wg, &pg, ctx, !opts.DontAddHash, !opts.DontAddSize)
	}
	// Deprecated
	if opts.ProgressBarWriter != nil {
		// Progress bar
		go func() {
			// Hide cursor
			fmt.Fprintf(opts.ProgressBarWriter, "\x1b[?25l")
			// Show cursor
			defer fmt.Fprintf(opts.ProgressBarWriter, "\x1b[?25h\n")
			// Last value, set it to something it can't be so we write at first.
			last := int64(0xfffffffffffffff)
			for {
				select {
				case <-ctx.Done():
					// Exit when context is done
					return
				default:
					// Check if the context is here
					n := pg.Load()
					if last != n {
						// New last value, write it. & clear the rest of the line.
						fmt.Fprintf(opts.ProgressBarWriter, "\r* %d of %d files have info\x1b[0K", n, len(files))
						last = n
					}
				}
			}
		}()
	}
	if opts.ProgressChan != nil {
		go func() {
			// Last value, set it to something it can't be so we write at first.
			last := int64(0xfffffffffffffff)
			for {
				select {
				case <-ctx.Done():
					// Send -1 as a done
					opts.ProgressChan <- -1
					return
				default:
					// Check if the context is here
					n := pg.Load()
					if last != n {
						opts.ProgressChan <- n
						last = n
					}
				}
			}
		}()
	}
	// Start sending files
	for _, v := range files {
		fileCh <- v
	}
	// All the files on the channel buffer will still be read even after we close.
	close(fileCh)
	// Wait for the goroutines to be done
	wg.Wait()
	// Cancel the context, which will stop the progress bar.
	cancel()
	for _, v := range files {
		if !opts.DontAddHash && v.hash == "" {
			// No hash added - this is a issue.
			return fmt.Errorf("not all files included hashes")
		}
		if !opts.DontAddSize && v.size == 0 {
			// No hash added - this is a issue.
			return fmt.Errorf("not all files included sizes")
		}
	}
	return nil
}
