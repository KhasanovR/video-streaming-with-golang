package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/gorilla/mux"
)

func main() {
	log.Println("App started")

	http.Handle("/", handlers())
	http.ListenAndServe(":8000", nil)
}

func handlers() *mux.Router {
	log.Println("Router setup")

	router := mux.NewRouter()
	router.HandleFunc("/", indexPage).Methods("GET")
	router.HandleFunc("/media/{mId:[0-9]+}/", uploadFileHandler).Methods("POST")
	router.HandleFunc("/media/{mId:[0-9]+}/", deleteFileHandler).Methods("DELETE")
	router.HandleFunc("/media/{mId:[0-9]+}/stream/", streamHandler).Methods("GET")
	router.HandleFunc("/media/{mId:[0-9]+}/stream/{segName:index[0-9]+.ts}", streamHandler).Methods("GET")

	return router
}

func indexPage(w http.ResponseWriter, r *http.Request) {
	log.Println("Router setup")

	http.ServeFile(w, r, "index.html")
}

func streamHandler(response http.ResponseWriter, request *http.Request) {
	log.Println("Stream handler started")

	vars := mux.Vars(request)
	mId, err := strconv.Atoi(vars["mId"])
	if err != nil {
		response.WriteHeader(http.StatusNotFound)
		return
	}

	segName, ok := vars["segName"]
	if !ok {
		mediaBase := getMediaBase(mId)
		m3u8Name := "index.m3u8"
		serveHlsM3u8(response, request, mediaBase, m3u8Name)
	} else {
		mediaBase := getMediaBase(mId)
		serveHlsTs(response, request, mediaBase, segName)
	}
}

func getMediaBase(mId int) string {
	log.Println("getMediaBase executed")

	mediaRoot := "assets/media"
	return fmt.Sprintf("%s/%d", mediaRoot, mId)
}

func serveHlsM3u8(w http.ResponseWriter, r *http.Request, mediaBase, m3u8Name string) {
	log.Println("serveHlsM3u8 executed")

	mediaFile := fmt.Sprintf("%s/hls/%s", mediaBase, m3u8Name)
	http.ServeFile(w, r, mediaFile)
	w.Header().Set("Content-Type", "application/x-mpegURL")
}

func serveHlsTs(w http.ResponseWriter, r *http.Request, mediaBase, segName string) {
	log.Println("serveHlsTs executed")

	mediaFile := fmt.Sprintf("%s/hls/%s", mediaBase, segName)
	http.ServeFile(w, r, mediaFile)
	w.Header().Set("Content-Type", "video/MP2T")
}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("uploadFileHandler executed")

	vars := mux.Vars(r)
	mId, err := strconv.Atoi(vars["mId"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	r.ParseMultipartForm(100 << 20)

	file, handler, err := r.FormFile("video")
	if err != nil {
		fmt.Errorf("%v", err)
		return
	}
	defer file.Close()

	if _, err := os.Stat(fmt.Sprintf("assets/media/%v/hls", mId)); os.IsNotExist(err) {
		err := os.MkdirAll(fmt.Sprintf("assets/media/%v/hls", mId), 0777)
		if err != nil {
			fmt.Errorf("%v", err)
			return
		}
	}

	dst, err := os.Create(fmt.Sprintf("./assets/media/%v/%s", mId, handler.Filename))
	defer dst.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	go segmentVideo(mId, handler.Filename)
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	resp := make(map[string]string)
	resp["message"] = "Successfully Uploaded File"
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		log.Fatalf("Error happened in JSON marshal. Err: %s", err)
	}
	w.Write(jsonResp)
	return
}

func segmentVideo(mId int, fileName string) {
	log.Println("segmentVideo executed")

	cmd := exec.Command("ffmpeg", "-i", fmt.Sprintf("assets/media/%v/%s", mId, fileName), "-level", "3.0", "-s", "1072x1920", "-start_number", "0", "-hls_time", "2", "-hls_list_size", "0", "-f", "hls", fmt.Sprintf("assets/media/%v/hls/index.m3u8", mId))

	//cmd := exec.Command(fmt.Sprintf("ffmpeg -i ./assets/media/%v/%s -profile:v baseline -level 3.0 -s 640x360 -start_number 0 -hls_time 10 -hls_list_size 0 -f hls ./assets/media/%v/hls/index.m3u8", mId, fileName, mId))

	err := cmd.Run()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
}

func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("deleteFileHandler executed")

	vars := mux.Vars(r)
	mId, err := strconv.Atoi(vars["mId"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if _, err := os.Stat(fmt.Sprintf("assets/media/%v", mId)); os.IsNotExist(err) {
		err := os.RemoveAll(fmt.Sprintf("assets/media/%v", mId))
		if err != nil {
			fmt.Errorf("%v", err)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	resp := make(map[string]string)
	resp["message"] = "Successfully Deleted File"
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		log.Fatalf("Error happened in JSON marshal. Err: %s", err)
	}
	w.Write(jsonResp)
	return
}
