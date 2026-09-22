# Testing

## Integration tests

Start MinIO:

```sh
colima start
docker run -d --name s3ry-minio -p 9000:9000 -e MINIO_ROOT_USER=minioadmin -e MINIO_ROOT_PASSWORD=minioadmin minio/minio:RELEASE.2025-09-07T16-13-09Z server /data
```

Set the endpoint and AWS credentials:

```sh
export S3RY_TEST_ENDPOINT=http://127.0.0.1:9000
export AWS_ACCESS_KEY_ID=minioadmin
export AWS_SECRET_ACCESS_KEY=minioadmin
make test-integration
```

`S3RY_TEST_BUCKET` defaults to `s3ry-it-<unix nanos>`. `S3RY_TEST_REGION` defaults to `us-east-1`.

Unit tests use gofakes3 in-process. Integration tests use MinIO pinned to the same image tag as CI; the `integration` CI job starts the identical container and runs `make test-integration`.
