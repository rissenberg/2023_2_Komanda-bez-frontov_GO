package api

import (
	"context"
	"fmt"
	"go-form-hub/internal/model"
	"go-form-hub/internal/repository"
	resp "go-form-hub/internal/services/service_response"
	"net/http"
	"time"
)

func AuthMiddleware(sessionRepository repository.SessionRepository, userRepository repository.UserRepository) func(next http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := r.Cookie("session_id")
			if err != nil {
				HandleError(w, fmt.Errorf("you have to log in or sign up to continue"), &resp.Response{StatusCode: http.StatusUnauthorized})
				return
			}

			sessionInDB, err := sessionRepository.FindByID(r.Context(), session.Value)
			if err != nil {
				HandleError(w, err, &resp.Response{StatusCode: http.StatusInternalServerError})
				return
			}

			if sessionInDB == nil {
				HandleError(w, fmt.Errorf("you have to log in or sign up to continue"), &resp.Response{StatusCode: http.StatusUnauthorized})
				return
			}

			if sessionInDB.CreatedAt+model.CookieExpiresIn.Milliseconds() < time.Now().UnixMilli() {
				HandleError(w, fmt.Errorf("session expired"), &resp.Response{StatusCode: http.StatusUnauthorized})
				return
			}

			currentUser, err := userRepository.FindByUsername(r.Context(), sessionInDB.Username)
			if err != nil {
				HandleError(w, err, &resp.Response{StatusCode: http.StatusInternalServerError})
				return
			}

			r = r.WithContext(context.WithValue(r.Context(), model.CurrentUserInContext, &model.UserGet{
				Username: currentUser.Username,
				Email:    currentUser.Email,
			}))
			next.ServeHTTP(w, r)
		})
	}
}
