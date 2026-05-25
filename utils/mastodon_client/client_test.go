package mastodonclient

import (
	"fmt"
	"os"
	"testing"
)

func TestUpload(t *testing.T) {
	client := &Client{
		Host:          "mstdn.jp",
		Cookie:        "cf_clearance=jRoAGmaaphMq1j2hus1sE.RpKvY1DcO4ns_NLnPU0nU-1751439221-1.2.1.1-FAUDdlZUMwMagXm.2VUjQ4ljkoGUE_J9YHkiiKCCKGiD_oWqmKlAH.JmyuZBUdgapK_rAstQjBmBXF0OOcUCkp20El1_GVL..tJaOxXm.w7mlSciQUPFSsmmsGIEBvktnbxBfs1zJKHAHiZNTpOEvgy9jDnBZYaJSrVKllcZfoUlnEPQTDcDXcgkth02XQhmyjNqwBWfGOGK5CfZ4l7E1Kg3WB3QL6DuTuZj0Vg0bwuJ1neUdY7uE7AJmLeCTR90dS4RPyOLM26VOyXPs_W4giFg7xGsK8ufRxvIgOIOoOpSmDvLHmOG5rU.5uY8tWnthxM.g1UQETFQZcAtLYewOeq7UygFgd0WtNbj3AIPrsIosIPPgX9EIxju9vW4TrPs; _session_id=eyJfcmFpbHMiOnsibWVzc2FnZSI6Iklqa3pObU0xWVRZMU1ETmlNRFF5WVROa05EbGxZamhrTVRreFpXUm1ZVFJpSWc9PSIsImV4cCI6IjIwMjYtMDctMTJUMTE6MDA6NTEuNjcxWiIsInB1ciI6ImNvb2tpZS5fc2Vzc2lvbl9pZCJ9fQ%3D%3D--cc37cb60117b64360f90a85d13c08b871eec4002; _mastodon_session=kDTUiwpszb1SPe7Ag%2FF96fT2GvO%2BhyuVCzQYjNomHQs6vcrt7O8OwsNunQ%2BxLLpO6Glv73pDyK1i5Ms%2B06RkOqp6x26E%2FLtTXl7fnfQZHnhLW677qRnm9z5VnhJWxsqGQJak8OXiPeq%2BQwAEZy3cEzmeyxlRmOiPsVzW1fJbYzliConw9QISyPkvRWXJqFBFzI5UhjudtNmJX7bMl3EnT%2B%2BuT9Diug42QIEK3fjJbdedXZ%2FYDvKhY72Nwb45kQf5KWhrFKVPqiZXr7SnM%2FNoY0s3dKc2oXMH38nu3o8783Wd5aoY4u7sx2yWJHr3gYw2MC6opj4nQehxe6kzgS%2BOi3pByh5btfpoJPOLLFVfrlSoVvt8jxFeWaDftuGhZgSzTJm3Z%2BoCmqVkusi%2B5%2FD1uwfkCMCLTuxL3dXZ1j0DgEdbLq0ZbTq3dBXE2DsEKK2emw%3D%3D--DK5FfCPcA89kRWNl--i%2FqDqQLj6pCfSGd%2BUlKyNQ%3D%3D",
		Authorization: "7UyxiXr0UsxaXWMOALFXoLEtXrbqVaI8SxlwW5Nhh40",
	}

	file, err := os.Open("/mnt/d/翻翻垃圾堆/Menhera-chan/3.jpg")
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()
	reader := file

	id, url, err := client.Upload(reader)
	if err != nil {
		t.Errorf("Upload failed: %v", err)
	}
	fmt.Println("Upload successful:", id, url)
}

func TestPostStatus(t *testing.T) {
	client := &Client{
		Host:          "mstdn.jp",
		Cookie:        "cf_clearance=jRoAGmaaphMq1j2hus1sE.RpKvY1DcO4ns_NLnPU0nU-1751439221-1.2.1.1-FAUDdlZUMwMagXm.2VUjQ4ljkoGUE_J9YHkiiKCCKGiD_oWqmKlAH.JmyuZBUdgapK_rAstQjBmBXF0OOcUCkp20El1_GVL..tJaOxXm.w7mlSciQUPFSsmmsGIEBvktnbxBfs1zJKHAHiZNTpOEvgy9jDnBZYaJSrVKllcZfoUlnEPQTDcDXcgkth02XQhmyjNqwBWfGOGK5CfZ4l7E1Kg3WB3QL6DuTuZj0Vg0bwuJ1neUdY7uE7AJmLeCTR90dS4RPyOLM26VOyXPs_W4giFg7xGsK8ufRxvIgOIOoOpSmDvLHmOG5rU.5uY8tWnthxM.g1UQETFQZcAtLYewOeq7UygFgd0WtNbj3AIPrsIosIPPgX9EIxju9vW4TrPs; _session_id=eyJfcmFpbHMiOnsibWVzc2FnZSI6Iklqa3pObU0xWVRZMU1ETmlNRFF5WVROa05EbGxZamhrTVRreFpXUm1ZVFJpSWc9PSIsImV4cCI6IjIwMjYtMDctMTJUMTE6MDA6NTEuNjcxWiIsInB1ciI6ImNvb2tpZS5fc2Vzc2lvbl9pZCJ9fQ%3D%3D--cc37cb60117b64360f90a85d13c08b871eec4002; _mastodon_session=kDTUiwpszb1SPe7Ag%2FF96fT2GvO%2BhyuVCzQYjNomHQs6vcrt7O8OwsNunQ%2BxLLpO6Glv73pDyK1i5Ms%2B06RkOqp6x26E%2FLtTXl7fnfQZHnhLW677qRnm9z5VnhJWxsqGQJak8OXiPeq%2BQwAEZy3cEzmeyxlRmOiPsVzW1fJbYzliConw9QISyPkvRWXJqFBFzI5UhjudtNmJX7bMl3EnT%2B%2BuT9Diug42QIEK3fjJbdedXZ%2FYDvKhY72Nwb45kQf5KWhrFKVPqiZXr7SnM%2FNoY0s3dKc2oXMH38nu3o8783Wd5aoY4u7sx2yWJHr3gYw2MC6opj4nQehxe6kzgS%2BOi3pByh5btfpoJPOLLFVfrlSoVvt8jxFeWaDftuGhZgSzTJm3Z%2BoCmqVkusi%2B5%2FD1uwfkCMCLTuxL3dXZ1j0DgEdbLq0ZbTq3dBXE2DsEKK2emw%3D%3D--DK5FfCPcA89kRWNl--i%2FqDqQLj6pCfSGd%2BUlKyNQ%3D%3D",
		Authorization: "7UyxiXr0UsxaXWMOALFXoLEtXrbqVaI8SxlwW5Nhh40",
	}

	client.PostStatus("测试状态", "", []string{}, false, "", "private", nil, "zh")
}
