package user

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5" // nolint:gosec
	"encoding/hex"
	"fmt"
	"io"
	"go-form-hub/internal/model"
	"go-form-hub/internal/config"
	"go-form-hub/internal/repository"
	resp "go-form-hub/internal/services/service_response"
	"net/http"

	validator "github.com/go-playground/validator/v10"
)

type Service interface {
	UserList(ctx context.Context) (*resp.Response, error)
	UserUpdate(ctx context.Context, user *model.UserUpdate) (*resp.Response, error)
	UserGet(ctx context.Context, id int64) (*resp.Response, error)
	UserGetAvatar(ctx context.Context, username string) (*resp.Response, error)
}

type userService struct {
	userRepository repository.UserRepository
	cfg            *config.Config
	validate       *validator.Validate
}

func NewUserService(userRepository repository.UserRepository, cfg *config.Config, validate *validator.Validate) Service {
	return &userService{
		userRepository: userRepository,
		cfg:			cfg,
		validate:       validate,
	}
}

func (s *userService) encryptPassword(pass string) (string, error) {
	keyBytes, err := hex.DecodeString(s.cfg.EncryptionKey)
	if err != nil {
		return "", fmt.Errorf("encrypt_password invalid hex-encoded key: %e", err)
	}

	if len(keyBytes) != 32 {
		return "", fmt.Errorf("invalid key length: expected 32 bytes, got %d", len(keyBytes))
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	hasher := md5.New() // nolint:gosec
	_, err = io.WriteString(hasher, pass)
	if err != nil {
		return "", err
	}
	nonce := hasher.Sum(nil)[:12]

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	ciphertext := aesgcm.Seal(nil, nonce, []byte(pass), nil)

	return hex.EncodeToString(nonce) + hex.EncodeToString(ciphertext), nil
}

func (s *userService) UserList(ctx context.Context) (*resp.Response, error) {
	var response model.UserList
	response.Users = make([]*model.UserGet, 0)

	users, err := s.userRepository.FindAll(ctx)
	if err != nil {
		return resp.NewResponse(http.StatusInternalServerError, nil), err
	}

	for _, user := range users {
		response.Users = append(response.Users, &model.UserGet{
			ID:        user.ID,
			Username:  user.Username,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
			Avatar:    user.Avatar,
		})
	}

	response.Count = len(users)
	return resp.NewResponse(http.StatusOK, response), nil
}

func (s *userService) UserGet(ctx context.Context, id int64) (*resp.Response, error) {
	user, err := s.userRepository.FindByID(ctx, id)
	if err != nil {
		return resp.NewResponse(http.StatusInternalServerError, nil), err
	}

	if user == nil {
		return resp.NewResponse(http.StatusNotFound, nil), nil
	}

	return resp.NewResponse(http.StatusOK, &model.UserGet{
		ID:        user.ID,
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Avatar:    user.Avatar,
	}), nil
}

func (s *userService) UserGetAvatar(ctx context.Context, username string) (*resp.Response, error) {
	user, err := s.userRepository.FindByUsername(ctx, username)
	if err != nil {
		return resp.NewResponse(http.StatusInternalServerError, nil), err
	}

	if user == nil {
		return resp.NewResponse(http.StatusNotFound, nil), nil
	}

	return resp.NewResponse(http.StatusOK, &model.UserAvatarGet{
		Username: user.Username,
		Avatar:   user.Avatar,
	}), nil
}

func (s *userService) UserUpdate(ctx context.Context, user *model.UserUpdate) (*resp.Response, error) {
	currentUser := ctx.Value(model.ContextCurrentUser).(*model.UserGet)
	if err := s.validate.Struct(user); err != nil {
		return resp.NewResponse(http.StatusBadRequest, nil), err
	}

	existing, err := s.userRepository.FindByID(ctx, currentUser.ID)
	if err != nil {
		return resp.NewResponse(http.StatusInternalServerError, nil), err
	}

	if existing == nil {
		return resp.NewResponse(http.StatusNotFound, nil), nil
	}

	encPassword, err := s.encryptPassword(user.Password)
	if err != nil {
		return resp.NewResponse(http.StatusInternalServerError, nil), err
	}

	if existing.Password != encPassword {
		return resp.NewResponse(http.StatusForbidden, nil), fmt.Errorf("invalid password")
	}

	if user.NewPassword == "" {
		user.NewPassword = user.Password
	}
	encNewPassword, err := s.encryptPassword(user.NewPassword)
	if err != nil {
		return resp.NewResponse(http.StatusInternalServerError, nil), err
	}

	err = s.userRepository.Update(ctx, existing.ID, &repository.User{
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Password:  encNewPassword,
		Email:     user.Email,
		Avatar:    user.Avatar,
	})
	if err != nil {
		return resp.NewResponse(http.StatusInternalServerError, nil), err
	}

	return resp.NewResponse(http.StatusOK, &model.UserGet{
		ID:		   currentUser.ID,
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Avatar:    user.Avatar,
	}), nil
}
