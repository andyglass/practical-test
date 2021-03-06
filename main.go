package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type Config struct {
	isMultiThread   bool
	Uri             string
	BucketName      string
	Prefix          string
	DestinationPath string
}

type Storage struct {
	Ctx    context.Context
	Client *storage.Client
	Config *Config
}

/*
	Create new storage config
*/
func NewConfig() *Config {
	// Custom usage decription
	flag.Usage = func() {
		fmt.Printf("Usage: %s [OPTIONS] bucket_name[/path][/file] path\n", os.Args[0])
		fmt.Println("\nArguments 'bucket_name' and 'path' are mandatory.")
		fmt.Println("Credentials must be provided via environment variable GOOGLE_APPLICATION_CREDENTIALS.")
		fmt.Println("Example: export GOOGLE_APPLICATION_CREDENTIALS=~/credentials.json")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
	}

	isMultiThread := flag.Bool("m", false, "Run command in multi-threading mode")
	flag.Parse()

	argLen := len(flag.Args())
	if argLen != 2 {
		fmt.Printf("Unexpected arguments count: %d instead of 2\n\n", argLen)
		flag.Usage()
		os.Exit(1)
	}

	uri := flag.Arg(0)
	destinationPath := flag.Arg(1)

	bucketName, prefix, err := parseGCSUrl(uri)
	if err != nil {
		exception(err)
	}

	return &Config{
		isMultiThread:   *isMultiThread,
		Uri:             uri,
		BucketName:      bucketName,
		Prefix:          prefix,
		DestinationPath: destinationPath,
	}
}

/*
	Create new storage object
*/
func NewStorage() *Storage {
	cfg := NewConfig()

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		exception(err)
	}

	return &Storage{
		Ctx:    ctx,
		Client: client,
		Config: cfg,
	}
}

/*
	List bucket objects by prefix
*/
func (s *Storage) ListObjects() ([]string, error) {
	ctx, cancel := context.WithTimeout(s.Ctx, time.Second*30)
	defer cancel()

	prefix := s.Config.Prefix
	// It helps to make relevant filtration by prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") && filepath.Ext(prefix) == "" {
		prefix += "/"
	}

	it := s.Client.Bucket(s.Config.BucketName).Objects(ctx, &storage.Query{
		Prefix: prefix,
	})

	var objects []string
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		objects = append(objects, attrs.Name)
	}

	if len(objects) == 0 {
		return nil, fmt.Errorf("no URLs matched: %s", s.Config.Uri)
	}

	return objects, nil
}

/*
	Download object from bucket
*/
func (s *Storage) DownloadObject(object string) error {
	ctx, cancel := context.WithTimeout(s.Ctx, time.Second*60)
	defer cancel()

	sr, err := s.Client.Bucket(s.Config.BucketName).Object(object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("Object(%q).NewReader: %v", object, err)
	}
	defer sr.Close()

	fpath := filepath.Join(s.Config.DestinationPath, object)

	// Create directory path if it does not exist (mkdir -p)
	if os.MkdirAll(filepath.Dir(fpath), os.ModePerm) != nil {
		return fmt.Errorf("os.MkdirAll: %v", err)
	}

	out, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("os.Create: %v", err)
	}
	defer out.Close()

	fmt.Printf("Copying %s => %s\n", object, fpath)

	_, err = io.Copy(out, sr)
	if err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	}

	return nil
}

/*
	Background worker for multi-threading mode
*/
func (s *Storage) DownloadObjectWithWorker(objects <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	// Read objects channel and download each object
	for obj := range objects {
		if err := s.DownloadObject(obj); err != nil {
			exception(err)
		}
	}
}

/*
	Validate and parse GCS uri ("gs://")
*/
func parseGCSUrl(uri string) (string, string, error) {
	const scheme = "gs://"

	if !strings.HasPrefix(uri, scheme) {
		return "", "", fmt.Errorf("scheme must be \"%s\": %s", scheme, uri)
	}

	u, err := url.Parse(uri)
	if err != nil {
		return "", "", fmt.Errorf("could not parse uri: %s", uri)
	}

	bucket := u.Host
	if bucket == "" {
		return "", "", fmt.Errorf("could not parse bucket name: %s", uri)
	}

	path := u.Path
	if path != "" {
		path = strings.Replace(path, "/", "", 1)
	}

	return bucket, path, nil
}

/*
	General exception wrapper
*/
func exception(err error) {
	fmt.Printf("CommandException: %v\n", err)
	os.Exit(1)
}

func main() {

	storage := NewStorage()
	defer storage.Client.Close()

	objects, err := storage.ListObjects()
	if err != nil {
		exception(err)
	}

	objectsCount := len(objects)

	// Multi-Threading mode
	if storage.Config.isMultiThread {
		var wg sync.WaitGroup
		objectsChan := make(chan string, objectsCount)

		workersCount := runtime.NumCPU() // <= workers pool size
		if objectsCount < workersCount { // <= reduces unnecessary workers
			workersCount = objectsCount
		}

		// Create background workers pool
		for w := 1; w <= workersCount; w++ {
			wg.Add(1)
			go storage.DownloadObjectWithWorker(objectsChan, &wg)
		}

		// Send objects to channel
		for j := 0; j < objectsCount; j++ {
			objectsChan <- objects[j]
		}

		close(objectsChan)
		wg.Wait()

	} else {
		// Usual mode
		for _, obj := range objects {
			if err := storage.DownloadObject(obj); err != nil {
				exception(err)
			}
		}
	}

	fmt.Printf("Operation completed over %d objects.\n", objectsCount)
}
