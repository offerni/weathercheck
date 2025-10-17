# Sistema de Consulta de Clima por CEP

Sistema simples de microserviços em Go que valida CEP e retorna informações de clima com rastreamento distribuído OpenTelemetry.

## Configuração

1. **Obter Chave da WeatherAPI**: Registre-se em [weatherapi.com](https://www.weatherapi.com/) para obter uma chave de API gratuita

2. **Configurar Ambiente**:

   ```bash
   cp .env.example .env
   # Edite .env e adicione sua WEATHER_API_KEY
   ```

3. **Iniciar Serviços**:
   ```bash
   docker-compose up --build -d
   ```

## Uso

**Requisição Válida**:

```bash
curl -X POST http://localhost:8080/weather \
  -H "Content-Type: application/json" \
  -d '{"cep": "17055250"}'
```

**Resposta**:

```json
{
  "city": "São Paulo",
  "temp_C": 25.0,
  "temp_F": 77.0,
  "temp_K": 298.0
}
```

## Serviços

- **Serviço A** (8080): Validação de CEP e encaminhamento de requisições
- **Serviço B** (8081): Orquestração de dados climáticos
- **Zipkin** (9411): Interface de rastreamento distribuído

## Testes

**CEP Válido**: `17055250` (São Paulo)
**CEP Inválido**: `123` (retorna 422)
**CEP Não Encontrado**: `99999999` (retorna 404)

Visualizar traces em: http://localhost:9411

---

# Weather Check System

Simple Go microservices system that validates CEP and returns weather information with OpenTelemetry distributed tracing.

## Setup

1. **Get WeatherAPI Key**: Register at [weatherapi.com](https://www.weatherapi.com/) for a free API key

2. **Configure Environment**:

   ```bash
   cp .env.example .env
   # Edit .env and add your WEATHER_API_KEY
   ```

3. **Start Services**:
   ```bash
   docker-compose up --build -d
   ```

## Usage

**Valid Request**:

```bash
curl -X POST http://localhost:8080/weather \
  -H "Content-Type: application/json" \
  -d '{"cep": "17055250"}'
```

**Response**:

```json
{
  "city": "São Paulo",
  "temp_C": 25.0,
  "temp_F": 77.0,
  "temp_K": 298.0
}
```

## Services

- **Service A** (8080): CEP validation and request forwarding
- **Service B** (8081): Weather data orchestration
- **Zipkin** (9411): Distributed tracing UI

## Testing

**Valid CEP**: `17055250` (São Paulo)
**Invalid CEP**: `123` (returns 422)
**Not Found**: `99999999` (returns 404)

View traces at: http://localhost:9411
