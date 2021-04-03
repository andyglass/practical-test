package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

func NewConfig() *Config {
	isMultiThread := flag.Bool("m", false, "Run command in multi-thread mode")
	flag.Parse()

	argLen := len(flag.Args())
	if argLen != 2 {
		exception(fmt.Errorf("unexpected arguments count: %d instead of 2", argLen))
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

func NewStorage() *Storage {

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		exception(err)
	}

	return &Storage{
		Ctx:    ctx,
		Client: client,
		Config: NewConfig(),
	}
}

func (s *Storage) ListObjects() ([]string, error) {
	ctx, cancel := context.WithTimeout(s.Ctx, time.Second*30)
	defer cancel()

	it := s.Client.Bucket(s.Config.BucketName).Objects(ctx, &storage.Query{
		Prefix: s.Config.Prefix,
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

func (s *Storage) DownloadObject(object string) error {
	ctx, cancel := context.WithTimeout(s.Ctx, time.Second*60)
	defer cancel()

	sr, err := s.Client.Bucket(s.Config.BucketName).Object(object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("Object(%q).NewReader: %v", object, err)
	}
	defer sr.Close()

	fpath := filepath.Join(s.Config.DestinationPath, object)

	err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm)
	if err != nil {
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

func exception(err error) {
	fmt.Printf("CommandException: %v\n", err)
	os.Exit(1)
}

/*
	TODO:
		- Optional: MultiThreading (-m)
*/

func main() {

	// url := "gs://online-infra-engineer-test/myd"

	storage := NewStorage()
	defer storage.Client.Close()

	objects, err := storage.ListObjects()
	if err != nil {
		exception(err)
	}

	for _, obj := range objects {
		err := storage.DownloadObject(obj)
		if err != nil {
			exception(err)
		}
	}

	fmt.Printf("Operation completed over %d objects.\n", len(objects))
}
