package internal

import (
	"context"
)

func DealUploadFilePath(ctx context.Context, uploadReader FileReader, postFileCh chan *UploadChanValue, cache bool) {
	defer close(postFileCh)

	uploadReader.Start(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			ur := uploadReader.Read(cache)
			postFileCh <- ur
			if ur.UploadExit {
				return
			}
		}
	}
}

type UploadChanValueReader interface {
	ReadUploadChanValue() UploadChanValue
}

type ValUploadChanValueReader struct {
	Value UploadChanValue
}

func (c *ValUploadChanValueReader) ReadUploadChanValue() UploadChanValue {
	return c.Value
}

type ChanUploadChanValueReader struct {
	Chan chan UploadChanValue
}

func (c *ChanUploadChanValueReader) ReadUploadChanValue() UploadChanValue {
	return <-c.Chan
}
