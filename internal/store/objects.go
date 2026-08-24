package store

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"soundledger/internal/domain"
	"strings"
)

type StoredObject struct {
	SHA256 string
	Path   string
	Size   int64
}

func (s *FileStore) PutObject(reader io.Reader, maxBytes int64) (StoredObject, error) {
	if maxBytes <= 0 {
		return StoredObject{}, fmt.Errorf("最大对象大小无效")
	}
	temp, err := os.CreateTemp(s.tmpDir, "audio-*.part")
	if err != nil {
		return StoredObject{}, err
	}
	tempName := temp.Name()
	keep := false
	defer func() {
		temp.Close()
		if !keep {
			_ = os.Remove(tempName)
		}
	}()
	h := sha256.New()
	limited := io.LimitReader(reader, maxBytes+1)
	size, err := io.Copy(io.MultiWriter(temp, h), limited)
	if err != nil {
		return StoredObject{}, err
	}
	if size == 0 {
		return StoredObject{}, fmt.Errorf("录音文件为空")
	}
	if size > maxBytes {
		return StoredObject{}, fmt.Errorf("录音文件超过 %d 字节限制", maxBytes)
	}
	if err = temp.Sync(); err != nil {
		return StoredObject{}, err
	}
	if err = temp.Close(); err != nil {
		return StoredObject{}, err
	}
	digest := hex.EncodeToString(h.Sum(nil))
	dir := filepath.Join(s.objectDir, digest[:2])
	if err = os.MkdirAll(dir, 0750); err != nil {
		return StoredObject{}, err
	}
	target := filepath.Join(dir, digest)
	if info, statErr := os.Stat(target); statErr == nil {
		if info.Size() != size {
			return StoredObject{}, fmt.Errorf("摘要对象大小冲突")
		}
		if err = verifyExistingObject(target, digest); err != nil {
			return StoredObject{}, err
		}
		return StoredObject{SHA256: digest, Path: relativeObjectPath(s.root, target), Size: size}, nil
	} else if !os.IsNotExist(statErr) {
		return StoredObject{}, statErr
	}
	if err = os.Rename(tempName, target); err != nil {
		return StoredObject{}, err
	}
	keep = true
	if err = syncDirectory(dir); err != nil {
		return StoredObject{}, err
	}
	return StoredObject{SHA256: digest, Path: relativeObjectPath(s.root, target), Size: size}, nil
}

func verifyExistingObject(path, expectedDigest string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != expectedDigest {
		return fmt.Errorf("已存在的内容寻址对象摘要校验失败")
	}
	return nil
}

func relativeObjectPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

func ValidateMediaType(value string) bool {
	base := strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	switch base {
	case "audio/wav", "audio/x-wav", "audio/flac", "audio/mpeg", "audio/ogg":
		return true
	}
	return false
}

func ValidateAudioContent(reader io.Reader, mediaType string) (io.Reader, error) {
	buffered := bufio.NewReader(reader)
	header, err := buffered.Peek(12)
	if err != nil {
		return nil, fmt.Errorf("录音内容太短，无法识别格式")
	}
	base := strings.ToLower(strings.TrimSpace(strings.Split(mediaType, ";")[0]))
	valid := false
	switch base {
	case "audio/wav", "audio/x-wav":
		valid = bytes.Equal(header[:4], []byte("RIFF")) && bytes.Equal(header[8:12], []byte("WAVE"))
	case "audio/flac":
		valid = bytes.Equal(header[:4], []byte("fLaC"))
	case "audio/ogg":
		valid = bytes.Equal(header[:4], []byte("OggS"))
	case "audio/mpeg":
		valid = bytes.Equal(header[:3], []byte("ID3")) || (header[0] == 0xff && header[1]&0xe0 == 0xe0)
	}
	if !valid {
		return nil, fmt.Errorf("录音文件头与 mediaType 不匹配")
	}
	return buffered, nil
}

func (s *FileStore) VerifyClips(clips []domain.AudioClip) []string {
	issues := []string{}
	for _, clip := range clips {
		if err := s.verifyClip(clip); err != nil {
			issues = append(issues, fmt.Sprintf("片段 %s 对象校验失败：%v", clip.ID, err))
		}
	}
	return issues
}

func (s *FileStore) verifyClip(clip domain.AudioClip) error {
	clean := filepath.Clean(filepath.FromSlash(clip.ObjectPath))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("对象路径越界")
	}
	path := filepath.Join(s.root, clean)
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("对象不是普通文件")
	}
	if info.Size() != clip.ByteSize {
		return fmt.Errorf("大小不一致，记录为 %d，实际为 %d", clip.ByteSize, info.Size())
	}
	hash := sha256.New()
	count, err := io.Copy(hash, file)
	if err != nil {
		return err
	}
	if count != clip.ByteSize {
		return fmt.Errorf("读取字节数不一致")
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != clip.SHA256 {
		return fmt.Errorf("SHA-256 不一致")
	}
	if filepath.Base(path) != clip.SHA256 {
		return fmt.Errorf("对象文件名与摘要不一致")
	}
	return nil
}
