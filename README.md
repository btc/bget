To run this:
```
go install github.com/btc/bget
bget
```

To specify the number of chunks to download in parallel:
```
bget -n 1000
```

To override the URL:
```
bget -url MY_URL
```
-----

The implementation is split into these parts:

`main.go` handles flags, calls MultiSourceGet, and saves the downloaded file to PWD.

`multisource_get.go` implements MultiSourceGet which sets up the download and delegates work.

`worker.go` implements HTTP range requests.

`file_download.go` is home to `FileDownload`, which encapsulates the shared state of the download, storing chunks and tracking the completion of ranges.

`main_test.go` contains a variety of tests. Some tests are exploratory: probing various file servers to reveal how responses can differ across implementations in the wild. These ones hit the network. Other tests are unit tests intended to mitigate risk.
