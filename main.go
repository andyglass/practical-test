package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type Config struct {
	BucketName      string
	Prefix          string
	DestinationPath string
}

type Storage struct {
	Client *storage.Client
}

func NewStorage() *Storage {

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	return &Storage{
		Client: client,
	}
}

func (s *Storage) ListObjects(bucket, prefix string) ([]string, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	it := s.Client.Bucket(bucket).Objects(ctx, &storage.Query{
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
	return objects, nil
}

func (s *Storage) DownloadObject(bucket, object, rootPath string) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	sr, err := s.Client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("Object(%q).NewReader: %v", object, err)
	}
	defer sr.Close()

	data, err := ioutil.ReadAll(sr)
	if err != nil {
		return fmt.Errorf("ioutil.ReadAll: %v", err)
	}

	path := filepath.Join(rootPath, object)

	err = os.MkdirAll(filepath.Dir(path), os.ModePerm)
	if err != nil {
		return fmt.Errorf("os.MkdirAll: %v", err)
	}

	err = ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("ioutil.WriteFile: %v", err)
	}

	log.Printf("%s => %s", object, path)

	return nil
}

func parseGCSUrl(uri string) (string, error) {
	const prefix = "gs://"

	if !strings.HasPrefix(uri, prefix) {
		return "", fmt.Errorf("parse GCS URI %q: scheme must be %q", uri, prefix)
	}

	object := uri[len(prefix):]
	if strings.IndexByte(object, '/') == -1 {
		return "", fmt.Errorf("parse GCS URI %q: no object name", uri)
	}

	return object, nil
}

/*
	TODO:
		- Return Bucket name and object prefix path separately
*/

func main() {

	bucketName := "online-infra-engineer-test"
	prefix := "mydir"
	path := "folder/newdir"

	// log.Print(parseGCSUrl("gs://online-infra-engineer-test/mydir"))

	storage := NewStorage()
	defer storage.Client.Close()

	objects, err := storage.ListObjects(bucketName, prefix)
	if err != nil {
		log.Fatal(err)
	}

	if len(objects) == 0 {
		log.Print("Objects not found")
		return
	}

	for _, obj := range objects {
		err := storage.DownloadObject(bucketName, obj, path)
		if err != nil {
			log.Print(err)
		}
	}
}
