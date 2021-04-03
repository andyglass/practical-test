# practical-test

## Usage

```bash
Usage: ./gcs-cp [OPTIONS] bucket_name[/path][/file] path

Arguments 'bucket_name' and 'path' are mandatory.
Credentials must be provided via environment variable GOOGLE_APPLICATION_CREDENTIALS.
Example: export GOOGLE_APPLICATION_CREDENTIALS=~/credentials.json

Options:
  -m    Run command in multi-threading mode
```

### From source

Provide GCP credentials file:
```bash
export GOOGLE_APPLICATION_CREDENTIALS="$PWD/credentials.json"
```

Run code from source:
```bash
go run main.go -h
```

Build project executable file:
```bash
go build -o ./gcs-cp main.go
./gcs-cp -h
```

### Docker

Build docker image:
```bash
docker build -t gcs-cp:latest .
```

Run docker container:
```bash
docker run --rm -v $PWD:/work -w /work \
  -e GOOGLE_APPLICATION_CREDENTIALS="./credentials.json" gcs-cp:latest
```