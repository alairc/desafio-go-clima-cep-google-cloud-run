// Package temperature implementa as conversões de temperatura exigidas pelo
// desafio. É um núcleo puro: sem I/O e sem dependências externas.
package temperature

import "math"

// kelvinOffset é o deslocamento usado na conversão Celsius -> Kelvin.
//
// O enunciado especifica K = C + 273. O JSON de exemplo do contrato mostra
// 28.5 C -> 301.65 K, o que corresponderia a C + 273.15. Seguimos a fórmula
// textual do requisito (273); trocar aqui é suficiente para adotar 273.15.
const kelvinOffset = 273.0

// Temperatures agrupa a mesma medição nas três escalas.
type Temperatures struct {
	Celsius    float64
	Fahrenheit float64
	Kelvin     float64
}

// CelsiusToFahrenheit aplica F = C * 1.8 + 32.
func CelsiusToFahrenheit(celsius float64) float64 {
	return round2(celsius*1.8 + 32)
}

// CelsiusToKelvin aplica K = C + 273.
func CelsiusToKelvin(celsius float64) float64 {
	return round2(celsius + kelvinOffset)
}

// FromCelsius monta as três escalas a partir de uma medição em Celsius.
func FromCelsius(celsius float64) Temperatures {
	return Temperatures{
		Celsius:    round2(celsius),
		Fahrenheit: CelsiusToFahrenheit(celsius),
		Kelvin:     CelsiusToKelvin(celsius),
	}
}

// round2 arredonda para duas casas decimais. Sem isso a aritmética de ponto
// flutuante vaza para a resposta JSON: 28.5 * 1.8 + 32 produz 83.30000000000001.
func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
