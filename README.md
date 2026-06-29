# ociregistry

A tiny Go library for pulling OCI images from Docker Hub and extracting their layers to disk.

## Install

```sh
go get github.com/4n4k1n/ociregistry
```

## Usage

```go
res, err := ociregistry.Pull("alpine", "latest", "./out")
if err != nil {
	log.Fatal(err)
}

fmt.Println(res.Config.Cmd)   // image command
for _, l := range res.Layers { // extracted layer dirs, bottom to top
	fmt.Println(l.Dir)
}
```

`Pull(image, tag, dest)` accepts:

- **image** — `alpine`, `library/alpine`, or `user/repo`
- **tag** — `latest`, a version tag, or a digest (`sha256:...`)
- **dest** — directory to extract layers into

It returns the image's runtime config (`Cmd`, `Entrypoint`, `Env`, `WorkingDir`) and the ordered, extracted layers.

## Notes

- Resolves manifest lists to `linux/amd64`.
- Applies whiteouts and guards against path traversal during extraction.
- Layers already extracted are skipped on re-pull.
