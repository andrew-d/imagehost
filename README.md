# imagehost

## What Is It?

imagehost (original name, huh?) is a simple, self-hosted image upload service.
It receives uploaded files, optionally archives them to a private S3 bucket,
strips all metadata from the image and corrects rotation, and then uploads the
"clean" image to a public S3 bucket.  The public URL for the uploaded image is
then returned.

## Why Should I Use This?

- If you want a simple, self-hosted alternative to other image uploading services.
- If you care about removing metadata from images before sharing them.
- If you think it's nifty.

## How Do I Use This?

Currently, you need to compile from source - instructions for doing this are below.
In the future, there will likely be "releases" consisting of statically-linked
binaries that can simply be copied into place, configured, and run.

### Building From Source

You need the following tools installed:

- [Go](https://golang.org/)
- [godep](https://github.com/tools/godep)

There's a Makefile provided, or you can simply run
`godep go build -o build/imagehost .` in the root directory to build the binary.


### Configuration

Configuration is stored in a YAML file that is passed to the program upon startup.
An [example configuration](https://github.com/andrew-d/imagehost/blob/master/config.yaml)
is included, with comments explaining what each value does.  imagehost will also
verify the configuration upon startup, and try to provide a useful error if
anything is incorrect.

### Running

`./imagehost -c config.yaml`

That's it.

## Contributors

- Andrew Dunham (@andrew-d)
- Hans Neilsen (@hansnielsen)

## License

MIT
