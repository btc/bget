To run this:

1. Clone this repo.
1. `make run`

```
git clone git@github.com:btc/bget.git
```

```
make run
```

NB: Under the hood, make run is just:
```
run: bget
    ./bget
bget:
    go build .
```
-----

The implementation is split into these parts:

`main.go` handles flags, calls MultiSourceGet, and saves the downloaded file to PWD.

`multisource_get.go` implements MultiSourceGet which sets up the download and delegates work.

`worker.go` implements HTTP range requests.

`file_download.go` is home to `FileDownload`, which encapsulates the shared state of the download, storing chunks and tracking the completion of ranges.

`main_test.go` contains a variety of tests. Some tests are exploratory: probing various file servers to reveal how responses can differ across implementations in the wild. These ones hit the network. Other tests are unit tests intended to mitigate risk.
