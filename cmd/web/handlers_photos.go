package main

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"seemyfamily.jmetzg11/internal/images"
	"seemyfamily.jmetzg11/internal/models"
)

const maxUploadBytes = 15 << 20

func (app *application) renderPhotos(w http.ResponseWriter, r *http.Request, id int, form photoForm, status int) {
	person, err := app.people.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	photos, err := app.photos.ByPerson(r.Context(), id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	data := app.newTemplateData(r)
	data.Page = "person"
	data.Person = person
	data.Photos = photos
	data.PhotoForm = form

	app.render(w, r, status, "photos.html", data)
}

func (app *application) gallery(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	app.renderPhotos(w, r, id, photoForm{}, http.StatusOK)
}

func (app *application) upload(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

	err = r.ParseMultipartForm(maxUploadBytes)
	if err != nil {
		var tooBig *http.MaxBytesError

		form := photoForm{Error: "Could not read the upload."}
		if errors.As(err, &tooBig) {
			form.Error = "That file is too large. Photos have to be under 15MB."
		}

		app.renderPhotos(w, r, id, form, http.StatusUnprocessableEntity)
		return
	}
	defer r.MultipartForm.RemoveAll()

	form := photoForm{Description: strings.TrimSpace(r.PostForm.Get("description"))}
	if len(form.Description) > 255 {
		form.Error = "Description cannot be more than 255 characters long."
		app.renderPhotos(w, r, id, form, http.StatusUnprocessableEntity)
		return
	}

	file, _, err := r.FormFile("photo")
	if err != nil {
		form.Error = "Choose a photo to upload."
		app.renderPhotos(w, r, id, form, http.StatusUnprocessableEntity)
		return
	}
	defer file.Close()

	original, err := io.ReadAll(file)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	resized, err := images.Resize(original)
	if err != nil {
		form.Error = "That file could not be read as an image. JPEG and PNG only — an iPhone HEIC photo has to be converted first."
		app.renderPhotos(w, r, id, form, http.StatusUnprocessableEntity)
		return
	}

	key := strconv.Itoa(id) + "/" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".jpeg"

	err = app.bucket.Put(r.Context(), key, "image/jpeg", resized)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	user, _ := userFromContext(r)

	err = app.photos.Insert(r.Context(), id, key, form.Description, user.Name)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	http.Redirect(w, r, "/person/"+strconv.Itoa(id)+"/photos", http.StatusSeeOther)
}
