package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type CEPRequest struct {
	CEP string `json:"cep"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

type WeatherResponse struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

type ViaCEPResponse struct {
	CEP         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Complemento string `json:"complemento"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	UF          string `json:"uf"`
	IBGE        string `json:"ibge"`
	GIA         string `json:"gia"`
	DDD         string `json:"ddd"`
	SIAFI       string `json:"siafi"`
	Erro        bool   `json:"erro,omitempty"`
}

type WeatherAPIResponse struct {
	Location struct {
		Name string `json:"name"`
	} `json:"location"`
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
}

var tracer oteltrace.Tracer

func initTracer() func() {
	// Create Zipkin exporter
	exporter, err := zipkin.New("http://zipkin:9411/api/v2/spans")
	if err != nil {
		log.Fatalf("Failed to create Zipkin exporter: %v", err)
	}

	// Create resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("service-b"),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create resource: %v", err)
	}

	// Create tracer provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	tracer = otel.Tracer("service-b")

	return func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}
}

func getCityFromCEP(ctx context.Context, cep string) (*ViaCEPResponse, error) {
	ctx, span := tracer.Start(ctx, "get-city-from-cep")
	defer span.End()

	span.SetAttributes(attribute.String("cep", cep))

	client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	url := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	var cepData ViaCEPResponse
	if err := json.Unmarshal(body, &cepData); err != nil {
		span.RecordError(err)
		return nil, err
	}

	if cepData.Erro {
		span.SetAttributes(attribute.Bool("cep.not_found", true))
		return nil, fmt.Errorf("CEP not found")
	}

	span.SetAttributes(attribute.String("city", cepData.Localidade))
	return &cepData, nil
}

func getWeather(ctx context.Context, city string) (*WeatherAPIResponse, error) {
	ctx, span := tracer.Start(ctx, "get-weather")
	defer span.End()

	span.SetAttributes(attribute.String("city", city))

	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" {
		err := fmt.Errorf("WEATHER_API_KEY environment variable not set")
		span.RecordError(err)
		return nil, err
	}

	client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	url := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%s&q=%s", apiKey, city)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	var weatherData WeatherAPIResponse
	if err := json.Unmarshal(body, &weatherData); err != nil {
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(attribute.Float64("temperature.celsius", weatherData.Current.TempC))
	return &weatherData, nil
}

func convertTemperatures(celsius float64) (float64, float64, float64) {
	fahrenheit := celsius*1.8 + 32
	kelvin := celsius + 273
	return celsius, fahrenheit, kelvin
}

func weatherHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "weather-handler")
	defer span.End()

	// Parse request body
	var req CEPRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		span.RecordError(err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "invalid zipcode"})
		return
	}

	span.SetAttributes(attribute.String("cep", req.CEP))

	// Get city from CEP
	cepData, err := getCityFromCEP(ctx, req.CEP)
	if err != nil {
		span.RecordError(err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "can not find zipcode"})
		return
	}

	// Get weather data
	weatherData, err := getWeather(ctx, cepData.Localidade)
	if err != nil {
		span.RecordError(err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "failed to get weather data"})
		return
	}

	// Convert temperatures
	tempC, tempF, tempK := convertTemperatures(weatherData.Current.TempC)

	response := WeatherResponse{
		City:  cepData.Localidade,
		TempC: tempC,
		TempF: tempF,
		TempK: tempK,
	}

	span.SetAttributes(
		attribute.String("response.city", response.City),
		attribute.Float64("response.temp_c", response.TempC),
		attribute.Float64("response.temp_f", response.TempF),
		attribute.Float64("response.temp_k", response.TempK),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Initialize tracing
	shutdown := initTracer()
	defer shutdown()

	// Setup Chi router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Add OpenTelemetry middleware
	r.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "service-b")
	})

	// Routes
	r.Post("/weather", weatherHandler)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	fmt.Println("Service B starting on port 8081")
	log.Fatal(http.ListenAndServe(":8081", r))
}
