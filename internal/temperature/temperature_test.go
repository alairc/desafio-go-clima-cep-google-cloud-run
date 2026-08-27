package temperature

import "testing"

func TestCelsiusToFahrenheit(t *testing.T) {
	tests := []struct {
		name    string
		celsius float64
		want    float64
	}{
		{"exemplo do contrato", 28.5, 83.3},
		{"zero absoluto em celsius", 0, 32},
		{"negativo", -10, 14},
		{"ponto de encontro das escalas", -40, -40},
		{"agua fervendo", 100, 212},
		{"fracionado", 21.7, 71.06},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CelsiusToFahrenheit(tt.celsius); got != tt.want {
				t.Errorf("CelsiusToFahrenheit(%v) = %v, quer %v", tt.celsius, got, tt.want)
			}
		})
	}
}

func TestCelsiusToKelvin(t *testing.T) {
	tests := []struct {
		name    string
		celsius float64
		want    float64
	}{
		{"exemplo do contrato", 28.5, 301.5},
		{"zero celsius", 0, 273},
		{"negativo", -273, 0},
		{"fracionado", 21.7, 294.7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CelsiusToKelvin(tt.celsius); got != tt.want {
				t.Errorf("CelsiusToKelvin(%v) = %v, quer %v", tt.celsius, got, tt.want)
			}
		})
	}
}

func TestFromCelsius(t *testing.T) {
	got := FromCelsius(28.5)
	want := Temperatures{Celsius: 28.5, Fahrenheit: 83.3, Kelvin: 301.5}

	if got != want {
		t.Errorf("FromCelsius(28.5) = %+v, quer %+v", got, want)
	}
}

// Garante que a aritmética de ponto flutuante não vaza para a resposta.
func TestFromCelsiusArredondaDuasCasas(t *testing.T) {
	if got := FromCelsius(28.5).Fahrenheit; got != 83.3 {
		t.Errorf("Fahrenheit = %v (%.17f), quer exatamente 83.3", got, got)
	}

	if got := FromCelsius(33.333).Celsius; got != 33.33 {
		t.Errorf("Celsius = %v, quer 33.33", got)
	}
}
