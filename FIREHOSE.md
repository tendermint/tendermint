# How to start extractor

Configuration for Firehose can be provide in `config.toml` file.

# Firehose stream

Streams can be processed just once using `STDOUT` option, but to
increase efficiency suggest to use is bundle writer, which stores
block stream in flat files.

# Description of configuration

`enabled` - boolean to enable or disable Firehose
`output_file` - `STDOUT` or `path/to/output/files/stream.log` to specify
location and name of stream output files
`bundle` - boolean to enable or disable bundle writer (recommended)
`bundle_size` - size of bundle chunk, ex. 1000
`start_height` - block height to start extractor
`end_height` - end height to stop extractor
`home` - root directory flag for output files

### Resources

More detailed documentation:

- [The Graph](https://thegraph.com/docs/en/)
- [Firehose](https://firehose.streamingfast.io/)
