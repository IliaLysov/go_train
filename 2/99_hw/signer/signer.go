package main

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

// сюда писать код

func ExecutePipeline(jobs ...job) {
	var in = make(chan interface{})
	wg := new(sync.WaitGroup)

	for _, j := range jobs {
		wg.Add(1)
		out := make(chan interface{})
		go func(j job, in, out chan interface{}) {
			defer close(out)
			defer wg.Done()
			j(in, out)
		}(j, in, out)
		in = out
	}

	wg.Wait()
}

func SingleHash(in, out chan interface{}) {
	wg := new(sync.WaitGroup)
	mu := new(sync.Mutex)
	for data := range in {
		wg.Add(1)
		go func(data interface{}) {
			defer wg.Done()
			if num, ok := data.(int); ok {
				str := strconv.Itoa(num)
				var md5 string
				md5done := make(chan struct{})
				go func() {
					mu.Lock()
					md5 = DataSignerMd5(str)
					mu.Unlock()
					close(md5done)
				}()
				crcChan := make(chan string)
				go func() {
					crcChan <- DataSignerCrc32(str)
				}()
				<-md5done
				crcMd5 := DataSignerCrc32(md5)
				crc := <-crcChan
				out <- crc + "~" + crcMd5
			}

		}(data)
	}
	wg.Wait()
}

func MultiHash(in, out chan interface{}) {
	wg := new(sync.WaitGroup)
	for data := range in {
		wg.Add(1)
		go func(data interface{}) {
			defer wg.Done()
			if str, ok := data.(string); ok {
				wg := new(sync.WaitGroup)
				var results [6]string
				for th := 0; th < 6; th++ {
					wg.Add(1)
					go func(th int) {
						defer wg.Done()
						results[th] = DataSignerCrc32(strconv.Itoa(th) + str)
					}(th)
				}
				wg.Wait()
				out <- strings.Join(results[:], "")
			}
		}(data)
	}
	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	var res []string
	for data := range in {
		if str, ok := data.(string); ok {
			res = append(res, str)
		}
	}
	sort.Strings(res)
	out <- strings.Join(res, "_")
}
