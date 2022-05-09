package application

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/omnibuildplatform/OmniRepository/app"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

type RepoMonitor struct {
	app.UnimplementedRepoServerServer
}

func (s *RepoMonitor) CallLoadFrom(ctx context.Context, in *app.RepRequest) (*app.RepResponse, error) {

	return nil, status.Errorf(codes.Unimplemented, "method CallLoadFrom not implemented")

}

func downLoadImages(image *app.Images, fullPath string) {
	image.Status = ImageStatusStart
	var response *http.Response
	defer func(status *string) {
		// update the image status at last
		image.UpdateTime = time.Now().In(app.CnTime)
		image.Status = *status
		err := app.UpdateImagesStatus(image)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("UpdateImagesStatus id:[%d] ,status:[%s],sourceurl:[%s] Error:%s", image.ID, image.Status, image.SourceUrl, err))
		}

		managerConf := app.Config.StringMap("manager")

		callbackURL := managerConf["callBackUrl"]
		if os.Getenv("CALLBACK_URL") != "" {
			callbackURL = os.Getenv("CALLBACK_URL")
		}

		callbackurl := fmt.Sprintf(callbackURL, image.ID, image.Status)
		response, err = http.Get(callbackurl)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("UpdateImagesStatus callback err:%s", err))
			return
		}
		if response.StatusCode != 200 {
			responseBody, _ := ioutil.ReadAll(response.Body)
			app.Logger.Error(fmt.Sprintf("downLoadImages Callback Error:%s ", string(responseBody)))
		}

		response.Body.Close()
	}(&image.Status)
	// var err error
	request, err := http.NewRequest(http.MethodGet, image.SourceUrl, nil)
	if err != nil {
		image.Status = ImageStatusFailed
		return
	}
	request.Header.Add("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		image.Status = ImageStatusFailed
		return
	}
	defer response.Body.Close()

	var savefile *os.File
	savefile, err = os.Create(fullPath)
	if err != nil {
		fmt.Println(err, "--------------- os.Create(fullPath): ", fullPath)
		image.Status = ImageStatusFailed
		return
	}
	defer savefile.Close()
	_, err = io.Copy(savefile, response.Body)
	if err != nil {
		fmt.Println(err, "--------------- os.Copy(fullPath): ", fullPath)

		image.Status = ImageStatusFailed
		return
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, savefile); err != nil {
		fmt.Println(err, "--------------- os.Copy(): ", fullPath)
		image.Status = ImageStatusFailed
		return
	}
	checksumValue := fmt.Sprintf("%X", hash.Sum(nil))
	if image.Checksum != checksumValue {
		err = fmt.Errorf("file's md5 not equal checkSum ")
		os.Remove(fullPath)
		fmt.Println(image.Checksum, "---------------Checksum: ", checksumValue)
		image.Status = ImageStatusFailed
		return
	}
	image.Status = ImageStatusDone
	return
}
