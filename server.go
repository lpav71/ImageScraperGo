package main

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	_ "io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"golang.org/x/net/html"
)

type ImageData struct {
	URL    string
	Width  int
	Height int
	Size   int64
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler).Methods("GET")
	r.HandleFunc("/go", GoHandler).Methods("POST")
	http.Handle("/", r)
	fmt.Println("Server listening on http://localhost:8081")
	http.ListenAndServe(":8081", nil)
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `<html>
 <head>
  <title>Image Scraper</title>
 </head>
 <body>
  <form action="/go" method="post">
   URL: <input type="text" name="url">
   <button type="submit">Go</button>
  </form>
 </body>
 </html>`)
}

// GoHandler обрабатывает HTTP-запросы, извлекает изображения с указанного URL и отображает результат.
func GoHandler(w http.ResponseWriter, r *http.Request) {
	// Закрываем тело запроса после завершения функции.
	defer r.Body.Close()

	// Получаем значение параметра 'url' из формы запроса.
	inputURL := r.FormValue("url")

	// Извлекаем изображения и их общий размер с указанного URL.
	images, totalSize, err := fetchImages(inputURL)
	if err != nil {
		// В случае ошибки при извлечении изображений возвращаем внутреннюю ошибку сервера.
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Отображаем результат, используя извлеченные изображения и их общий размер.
	renderResult(w, images, totalSize)
}

// fetchImages загружает изображения с указанной страницы и возвращает их данные и общий размер.
func fetchImages(pageURL string) ([]ImageData, int64, error) {
	// Отправляем HTTP GET запрос на указанный URL.
	resp, err := http.Get(pageURL)
	if err != nil {
		return nil, 0, err
	}
	// Закрываем тело ответа после завершения функции.
	defer resp.Body.Close()

	// Парсим HTML-документ из тела ответа.
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	// Извлекаем URL-адреса изображений из HTML-документа.
	imageURLs := extractImageURLs(doc, pageURL)
	var images []ImageData
	var totalSize int64

	// Проходим по каждому URL изображения и загружаем его данные.
	for _, imgURL := range imageURLs {
		imgData, err := fetchImage(imgURL)
		if err == nil {
			// Если загрузка изображения успешна, добавляем его данные в список и увеличиваем общий размер.
			images = append(images, imgData)
			totalSize += imgData.Size
		}
	}

	// Возвращаем список данных изображений и общий размер.
	return images, totalSize, nil
}

func extractImageURLs(n *html.Node, baseURL string) []string {
	// Слайс для хранения найденных URL изображений
	var imageURLs []string

	// Определяем функцию crawler для рекурсивного обхода дерева узлов HTML
	var crawler func(*html.Node)
	crawler = func(node *html.Node) {
		// Проверяем, является ли текущий узел элементом <img>
		if node.Type == html.ElementNode && node.Data == "img" {
			// Проходим по всем атрибутам элемента <img>
			for _, attr := range node.Attr {
				// Ищем атрибут "src", содержащий URL изображения
				if attr.Key == "src" {
					imgURL := attr.Val
					// Если URL не абсолютный (не начинается с "http"), преобразуем его в абсолютный
					if !strings.HasPrefix(imgURL, "http") {
						base, _ := url.Parse(baseURL)                // Парсим базовый URL
						ref, _ := url.Parse(imgURL)                  // Парсим относительный URL изображения
						imgURL = base.ResolveReference(ref).String() // Разрешаем относительный URL относительно базового
					}
					// Добавляем найденный URL изображения в слайс
					imageURLs = append(imageURLs, imgURL)
				}
			}
		}
		// Рекурсивно обходим всех потомков текущего узла
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			crawler(c)
		}
	}

	// Запускаем рекурсивный обход с корневого узла
	crawler(n)

	// Возвращаем слайс найденных URL изображений
	return imageURLs
}

// fetchImage получает изображение по заданному URL и возвращает информацию об изображении
// такую как URL, ширина, высота и размер файла.
func fetchImage(imgURL string) (ImageData, error) {
	// Отправляем HTTP GET запрос по URL
	resp, err := http.Get(imgURL)
	if err != nil {
		// Если произошла ошибка при отправке запроса, возвращаем пустую структуру ImageData и ошибку
		return ImageData{}, err
	}

	// Закрываем тело ответа, когда функция завершит выполнение, чтобы освободить ресурсы
	defer resp.Body.Close()

	// Декодируем изображение из тела ответа
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		// Если произошла ошибка при декодировании, возвращаем пустую структуру ImageData и ошибку
		return ImageData{}, err
	}

	// Получаем размер изображения из заголовка ответа и преобразуем его в целое число
	size, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		// Если произошла ошибка при преобразовании размера, возвращаем пустую структуру ImageData и ошибку
		return ImageData{}, err
	}

	// Возвращаем заполненную структуру ImageData
	return ImageData{
		URL:    imgURL,            // URL изображения
		Width:  img.Bounds().Dx(), // Ширина изображения
		Height: img.Bounds().Dy(), // Высота изображения
		Size:   size,              // Размер файла
	}, nil
}

func formatSize(size int64) string {
	return fmt.Sprintf("%.2f MB", float64(size)/1024/1024)
}

func renderResult(w http.ResponseWriter, images []ImageData, totalSize int64) {
	fmt.Fprintf(w, `<html>
 <head>
  <title>Image Scraper Result</title>
 </head>
 <body>
  <div>
   <h3>Найдено изображений: %d с общим объёмом %s</h3>
  </div>
  <div style="display: flex; flex-wrap: wrap;">`, len(images), formatSize(totalSize))
	for _, img := range images {
		fmt.Fprintf(w, `<div style="width: 25%%; padding: 5px;">

   <img src="%s" style="max-width: 100%%;">
   </div>`, img.URL)
	}
	fmt.Fprintf(w, `</div>
 </body>
 </html>`)
}
