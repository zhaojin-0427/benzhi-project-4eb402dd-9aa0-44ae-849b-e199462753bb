package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"soundledger/internal/application"
	"soundledger/internal/httpapi"
	"soundledger/internal/store"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Printf("声景证据册启动失败: %v", err)
		os.Exit(1)
	}
}

func run() error {
	flags := flag.NewFlagSet("soundledger", flag.ContinueOnError)
	addressFlag := flags.String("addr", defaultAddress, "回环监听地址")
	dataDir := flags.String("data-dir", "data", "持久化数据目录")
	selfcheck := flags.Bool("selfcheck", false, "运行真实 HTTP 主流程自检后退出")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	explicit := false
	flags.Visit(func(item *flag.Flag) {
		if item.Name == "addr" {
			explicit = true
		}
	})
	address, err := resolveAddress(*addressFlag, explicit)
	if err != nil {
		return err
	}
	actualDataDir := *dataDir
	cleanup := func() {}
	if *selfcheck {
		actualDataDir, err = os.MkdirTemp("", "soundledger-selfcheck-")
		if err != nil {
			return err
		}
		cleanup = func() { _ = os.RemoveAll(actualDataDir) }
	}
	defer cleanup()
	actualDataDir, err = filepath.Abs(actualDataDir)
	if err != nil {
		return err
	}
	repository, err := store.Open(actualDataDir)
	if err != nil {
		return fmt.Errorf("打开数据目录: %w", err)
	}
	app, err := application.New(application.Config{Store: repository, MaxUploadBytes: 64 << 20})
	if err != nil {
		return err
	}
	api := httpapi.New(app)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", address, err)
	}
	server := &http.Server{Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	serveError := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveError <- err
		}
		close(serveError)
	}()
	if *selfcheck {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		baseURL := "http://" + listener.Addr().String()
		checkErr := httpapi.RunSelfCheck(ctx, baseURL)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		if checkErr != nil {
			return checkErr
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		if err := <-serveError; err != nil {
			return err
		}
		fmt.Println("selfcheck 通过：批次创建、上传、双盲标注、争议仲裁、冻结、发布与证据查询均成功")
		return nil
	}
	log.Printf("声景证据册监听 %s，数据目录 %s", listener.Addr().String(), actualDataDir)
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalCtx.Done():
	case err := <-serveError:
		if err != nil {
			return err
		}
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
