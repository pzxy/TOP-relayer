export CGO_ENABLED="1"
export CGO_CFLAGS="-g -O -D__BLST_PORTABLE__"
go build -o xrelayer main.go


