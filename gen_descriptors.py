#!/usr/bin/env python

from __future__ import print_function

import os
import sys
import optparse
import mimetypes

template = """
package main

type AssetDescriptor struct {
\tPath string
\tMime string
}

func AssetDescriptors() []AssetDescriptor {
\treturn []AssetDescriptor{
%s
\t}
}
""".strip()


def mime(fname):
    if fname.endswith('.js'):
        return 'text/javascript'
    elif fname.endswith('.map'):
        # This could also be applicaton/octet-stream, but this:
        #   http://stackoverflow.com/questions/19911929/what-mime-type-should-i-use-for-source-map-files
        # says that Google's CDN returns application/json, so that's what we'll
        # do.
        return 'application/json'

    ty, _ = mimetypes.guess_type(fname)
    return ty


def main():
    if len(sys.argv) < 2:
        print("Usage: gen_assets.py <input dir> [debug]", file=sys.stderr)
        return 1

    parser = optparse.OptionParser()
    parser.add_option("-p", "--prefix", dest="prefix",
                      help="prefix to strip from input files")
    parser.add_option("--debug", dest="debug", action="store_true",
                      default=False, help="enable debug mode")

    options, args = parser.parse_args()

    lines = []
    for fname in args:
        if fname.startswith(options.prefix):
            fname = fname[len(options.prefix):]
        if not options.debug and fname.endswith('.map'):
            continue

        lines.append('\t\t{"%s", "%s"},' % (
            fname,
            mime(fname),
        ))

    output = template % ('\n'.join(lines),)
    print(output, end='')


if __name__ == "__main__":
    try:
        ret = main() or 0
    except KeyboardInterrupt:
        ret = 0

    sys.exit(ret)
