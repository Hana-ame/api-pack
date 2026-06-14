package exhentai_modify

// func TestRotator(rotator *IPRotator) {
// 	var wg sync.WaitGroup
// 	totalRequests := 5000 // 请求量大一点，测试负载均衡

// 	for i := 0; i < totalRequests; i++ {
// 		wg.Add(1)
// 		go func(reqNum int) {
// 			defer wg.Done()
// 			// 这里的 Fetch 会被均衡分发到 10 个 IP 上
// 			resp, err := rotator.Fetch("GET", "https://ifconfig.me/ip", nil, nil)
// 			if err != nil {
// 				log.Printf("Req %d error: %v", reqNum, err)
// 				return
// 			}
// 			defer resp.Body.Close()

// 			body, _ := io.ReadAll(resp.Body)
// 			fmt.Printf("%s\n", body)
// 		}(i + 1)

// 		if i%100 == 0 {
// 			time.Sleep(1 * time.Millisecond)
// 		}
// 	}

// 	wg.Wait()
// 	log.Println("Done. Waiting for cleanup...")
// 	time.Sleep(10 * time.Second)
// }
