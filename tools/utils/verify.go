package tools

import (
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	// "github.com/Hana-ame/fedi-antenna/actions/model"
	// "github.com/Hana-ame/fedi-antenna/core/dao"
	// "github.com/Hana-ame/fedi-antenna/core/utils"
	// "github.com/Hana-ame/fedi-antenna/core/utils"
	"github.com/Hana-ame/httpsig"
	"github.com/gin-gonic/gin"
)

func VerifyGin(c *gin.Context, body []byte, pubkeyRetriver func(id string) (crypto.PublicKey, error)) error {
	// verify
	if c.Request.Header.Get("Host") == "" || strings.Contains(c.Request.Header.Get("HOST"), ":8080") || strings.HasPrefix(c.Request.Header.Get("HOST"), "127.") {
		c.Request.Header.Set("Host", os.Getenv("HOST"))
	}
	if err := Verify(c.Request, pubkeyRetriver); err != nil {
		log.Println(err)
		return err
	}
	_, digest := parseDigest(c.GetHeader("Digest"))
	sha256 := sha256.Sum256([]byte(body))
	encoded := base64.StdEncoding.EncodeToString([]byte(sha256[:]))
	if digest != encoded {
		log.Printf("digest != encoded\n")
		return fmt.Errorf("digest != encoded")
	}
	return nil
}

// signature
// "SHA-256=8RIlimPwETDMkWQMI59d0gm9dqhzKGtX0CsEcahxxOE=" => "SHA-256", "8RIlimPwETDMkWQMI59d0gm9dqhzKGtX0CsEcahxxOE="
func parseDigest(d string) (algorithm, digest string) {
	arr := strings.SplitN(d, "=", 2)
	if len(arr) != 2 {
		return
	}
	return arr[0], arr[1]
}

func Verify(r *http.Request, publicKeyRetriever func(id string) (crypto.PublicKey, error)) (err error) {
	// defer func() {
	// 	if e := recover(); e != nil {
	// 		err = (fmt.Errorf("%s", e))
	// 	}
	// }()
	verifier, err := httpsig.NewVerifier(r)
	if err != nil {
		return err
	}
	var algo httpsig.Algorithm = parseAlgorithm(r.Header.Get("Signature"))
	var pubKeyID = parsePublicKeyID(r.Header.Get("Signature"))
	pubKey, err := publicKeyRetriever(pubKeyID)
	if err != nil {
		return
	}
	return verifier.Verify(pubKey, algo)
}

func parseAlgorithm(signature string) httpsig.Algorithm {
	for _, v := range strings.Split(signature, ",") {
		if strings.HasPrefix(v, "algorithm") {
			algorithm := strings.TrimPrefix(v, "algorithm=")
			algorithm = strings.TrimPrefix(algorithm, "\"")
			algorithm = strings.TrimSuffix(algorithm, "\"")

			return httpsig.Algorithm(algorithm)
		}
	}
	return ""
}

func parsePublicKeyID(signature string) (id string) {
	for _, v := range strings.Split(signature, ",") {
		if strings.HasPrefix(v, "keyId") {
			keyId := strings.TrimPrefix(v, "keyId=")
			keyId = strings.TrimPrefix(keyId, "\"")
			keyId = strings.TrimSuffix(keyId, "\"")
			keyId = strings.TrimSuffix(keyId, "#main-key")

			return keyId
		}
	}
	return ""
}
