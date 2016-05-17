package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/dustin/go-humanize"
	"github.com/evergreen-ci/evergreen/service"
	"github.com/evergreen-ci/evergreen/util"
)

// FetchCommand is used to fetch the source or artifacts associated with a task.
type FetchCommand struct {
	GlobalOpts *Options `no-flag:"true"`
	//Source     bool     `long:"source" description:"clones the source for the given task"`
	Artifacts bool   `long:"artifacts" description:"fetch artifacts for the task and all its recursive dependents"`
	Shallow   bool   `long:"shallow" description:"don't recursively download artifacts from dependency tasks"`
	NoPatch   bool   `long:"no-patch" description:"when using --source with a patch task, skip applying the patch"`
	TaskId    string `short:"t" long:"task" description:"task associated with the data to fetch" required:"true"`
}

func (fc *FetchCommand) Execute(args []string) error {
	ac, rc, _, err := getAPIClients(fc.GlobalOpts)
	if err != nil {
		return err
	}
	notifyUserUpdate(ac)

	task, err := rc.GetTask(fc.TaskId)
	if err != nil {
		return err
	}

	urls, err := getUrls(rc, task, false)
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	err = downloadUrls(wd, urls, 4)
	if err != nil {
		return err
	}
	return nil
}

func searchDependencies(rc *APIClient, seed *service.RestTask, found map[string]bool) ([]*service.RestTask, error) {
	out := []*service.RestTask{}
	for _, dep := range seed.DependsOn {
		if _, ok := found[dep.TaskId]; ok {
			continue
		}
		t, err := rc.GetTask(dep.TaskId)
		if err != nil {
			return nil, err
		}
		if t != nil {
			found[t.Id] = true
			out = append(out, t)
			more, err := searchDependencies(rc, t, found)
			if err != nil {
				return nil, err
			}
			out = append(out, more...)
			for _, d := range more {

				found[d.Id] = true

			}
		}
	}
	return out, nil
}

type artifactDownload struct {
	url  string
	path string
}

func getUrls(rc *APIClient, seed *service.RestTask, shallow bool) (chan artifactDownload, error) {
	allTasks := []*service.RestTask{seed}
	if !shallow {
		fmt.Println("Gathering dependencies")
		deps, err := searchDependencies(rc, seed, map[string]bool{})
		if err != nil {
			return nil, err
		}
		allTasks = append(allTasks, deps...)
	}

	urls := make(chan artifactDownload)
	go func() {
		for _, t := range allTasks {
			for _, f := range t.Files {
				fmt.Println("Found url", f.URL)
				urls <- artifactDownload{
					f.URL,
					fmt.Sprintf("%v_%v", t.BuildVariant, t.DisplayName),
				}
			}
		}
		close(urls)
	}()
	return urls, nil
}

func downloadUrls(root string, urls chan artifactDownload, workers int) error {
	if workers <= 0 {
		panic("invalid workers count")
	}
	wg := sync.WaitGroup{}
	errs := make(chan error)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			counter := 0
			for u := range urls {
				folder := filepath.Join(root, u.path)
				// backup plan in case we can't parse the file path out of the URL
				justFile := fmt.Sprintf("%v_%v", workerId, counter)
				parsedUrl, err := url.Parse(u.url)
				if err == nil {
					pathParts := strings.Split(parsedUrl.Path, "/")
					if len(pathParts) >= 1 {
						justFile = util.CleanForPath(pathParts[len(pathParts)-1])
					}
				}

				fileName := filepath.Join(folder, justFile)

				err = os.MkdirAll(folder, 0777)
				if err != nil {
					errs <- fmt.Errorf("Couldn't create output directory %v: %v", folder, err)
					continue
				}

				out, err := os.Create(fileName)
				if err != nil {
					errs <- fmt.Errorf("Couldn't download %v: %v", u.url, err)
					continue
				}
				resp, err := http.Get(u.url)
				if err != nil {
					errs <- fmt.Errorf("Couldn't download %v: %v", u.url, err)
					continue
				}
				length, _ := strconv.Atoi(resp.Header.Get("Content-Length"))
				sizeLog := ""
				if length > 0 {
					sizeLog = fmt.Sprintf(" (%s)", humanize.Bytes(uint64(length)))
				}

				fmt.Printf("(worker %v) Downloading to %v%s\n", workerId, justFile, sizeLog)
				//sizeTracker := util.SizeTrackingReader{0, resp.Body}
				_, err = io.Copy(out, resp.Body)
				if err != nil {
					errs <- fmt.Errorf("Couldn't download %v: %v", u.url, err)
					continue
				}
				resp.Body.Close()
				out.Close()
				counter++
			}
		}(i)
	}
	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	for {
		select {
		case <-wgDone:
			break
		case err := <-errs:
			fmt.Println("error: ", err)
		}
	}

}
