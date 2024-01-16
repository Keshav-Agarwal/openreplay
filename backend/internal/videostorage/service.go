package videostorage

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	config "openreplay/backend/internal/config/videostorage"
	"openreplay/backend/pkg/objectstorage"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Task struct {
	sessionID   string
	path        string
	name        string
	isBreakTask bool
}

func NewBreakTask() *Task {
	return &Task{isBreakTask: true}
}

type VideoStorage struct {
	cfg            *config.Config
	framerate      string
	objStorage     objectstorage.ObjectStorage
	sendToS3Tasks  chan *Task
	workersStopped chan struct{}
}

func New(cfg *config.Config, objStorage objectstorage.ObjectStorage) (*VideoStorage, error) {
	switch {
	case cfg == nil:
		return nil, fmt.Errorf("config is empty")
	case objStorage == nil:
		return nil, fmt.Errorf("object storage is empty")
	case cfg.VideoReplayFPS <= 0:
		return nil, fmt.Errorf("video replay fps is invalid: %d", cfg.VideoReplayFPS)
	}
	newStorage := &VideoStorage{
		cfg:            cfg,
		framerate:      strconv.Itoa(cfg.VideoReplayFPS),
		objStorage:     objStorage,
		sendToS3Tasks:  make(chan *Task, 1),
		workersStopped: make(chan struct{}),
	}
	go newStorage.runWorker()
	return newStorage, nil
}

func (v *VideoStorage) makeVideo(sessID uint64, filesPath string) error {
	files, _ := ioutil.ReadDir(filesPath)
	log.Printf("There are %d screenshot of session %d\n", len(files), sessID)

	// Try to call ffmpeg and print the result
	start := time.Now()
	sessionID := strconv.FormatUint(sessID, 10)
	imagesPath := "/mnt/efs/screenshots/" + sessionID + "/%06d.jpeg"
	videoPath := "/mnt/efs/screenshots/" + sessionID + "/replay.mp4"
	cmd := exec.Command("ffmpeg", "-y", "-f", "image2", "-framerate", v.framerate, "-start_number", "000000", "-i",
		imagesPath, "-vf", "scale=-2:1064", "-c:v", "libx264", "-preset", "ultrafast", "-crf", "23",
		videoPath)
	// ffmpeg -y -f concat -safe 0 -i 1699978964098_29-list -vf --pix_fmt yuv420p -preset ultrafast canvas.mp4

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Fatalf("Failed to execute command: %v, stderr: %v", err, stderr.String())
	}
	log.Printf("made video replay in %v", time.Since(start))
	v.sendToS3Tasks <- &Task{sessionID: sessionID, path: videoPath}
	return nil
}

func (v *VideoStorage) makeCanvasVideo(sessID uint64, filesPath, canvasMix string) error {
	files, err := ioutil.ReadDir(filesPath)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	log.Printf("There are %d mix lists of session %d\n", len(files), sessID)

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "-list") {
			continue
		}

		name := strings.TrimSuffix(file.Name(), "-list")
		mixList := fmt.Sprintf("%s%s-list", filesPath, name)
		videoPath := fmt.Sprintf("%s%s.mp4", filesPath, name)

		// Run ffmpeg to build video
		start := time.Now()
		sessionID := strconv.FormatUint(sessID, 10)

		cmd := exec.Command("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", mixList, "-vf", "pad=ceil(iw/2)*2:ceil(ih/2)*2", "-vsync", "vfr",
			"-pix_fmt", "yuv420p", "-preset", "ultrafast", videoPath)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		if err != nil {
			log.Fatalf("Failed to execute command: %v, stderr: %v", err, stderr.String())
		}
		log.Printf("made video replay in %v", time.Since(start))
		v.sendToS3Tasks <- &Task{sessionID: sessionID, path: videoPath, name: "/" + name + ".mp4"}
	}
	return nil
}

func (v *VideoStorage) sendToS3(task *Task) {
	start := time.Now()
	// Read video file from disk
	video, err := ioutil.ReadFile(task.path)
	if err != nil {
		log.Fatalf("Failed to read video file: %v", err)
	}
	// Upload video file to S3
	key := task.sessionID
	if task.name != "" {
		key += task.name
	} else {
		key += "/replay.mp4"
	}
	if err := v.objStorage.Upload(bytes.NewReader(video), key, "video/mp4", objectstorage.NoCompression); err != nil {
		log.Fatalf("Storage: start upload video replay failed. %s", err)
	}
	log.Printf("Video file (size: %d) uploaded successfully in %v", len(video), time.Since(start))
	return
}

func (v *VideoStorage) Process(sessID uint64, filesPath string, canvasMix string) error {
	if canvasMix != "" {
		return v.makeCanvasVideo(sessID, filesPath, canvasMix)
	}
	return v.makeVideo(sessID, filesPath)
}

func (v *VideoStorage) Wait() {
	v.sendToS3Tasks <- NewBreakTask()
	<-v.workersStopped
}

func (v *VideoStorage) runWorker() {
	for {
		select {
		case task := <-v.sendToS3Tasks:
			if task.isBreakTask {
				v.workersStopped <- struct{}{}
				continue
			}
			v.sendToS3(task)
		}
	}
}
