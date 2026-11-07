// Suggests a Category while the Usuario writes the description of an Expense
// (docs/pendientes.md, «Sugerir la Categoría a partir de la descripción»). It
// runs on the device: offline, as the Usuario types, and the description never
// leaves it. A wrong suggestion costs one tap, so it only has to be right often.

export type CategoryName =
  | 'food'
  | 'groceries'
  | 'transport'
  | 'lodging'
  | 'housing'
  | 'utilities'
  | 'entertainment'
  | 'shopping'
  | 'health'
  | 'gifts'
  | 'other'

// An Expense the Usuario already gave a Category, newest first.
export interface CategorizedExpense {
  description: string
  category: CategoryName
}

// Lower case, without accents or punctuation, and single spaces:
// '  Almuerzo   en El Corral!' is 'almuerzo en el corral'.
export function normalizeDescription(description: string): string {
  return description
    .normalize('NFD')
    .replace(/\p{M}/gu, '')
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]+/gu, ' ')
    .trim()
}

// The Usuario's own history first, since descriptions repeat («Netflix»,
// «Arriendo», «Mercado») and so it adapts to each person; then the keywords.
export function suggestCategory(description: string, history: readonly CategorizedExpense[]): CategoryName | undefined {
  const text = normalizeDescription(description)
  if (text.length < 2) return undefined
  return fromHistory(text, history) ?? fromKeywords(text)
}

// The words that say what an Expense was, without the ones that say nothing
// about it ('de', 'con', a number).
function meaningfulWords(text: string): string[] {
  return text.split(' ').filter((word) => word.length >= 3 && !stopWords.has(word) && !/^\d+$/.test(word))
}

const stopWords = new Set(['con', 'del', 'las', 'los', 'una', 'uno', 'para', 'por', 'que', 'mas', 'and', 'the'])

function fromHistory(text: string, history: readonly CategorizedExpense[]): CategoryName | undefined {
  const same = history.filter((expense) => normalizeDescription(expense.description) === text)
  if (same.length) return mostUsed(same)

  // Almost the same: one description holds every meaningful word of the other,
  // as «Netflix noviembre» does with «Netflix».
  const words = new Set(meaningfulWords(text))
  if (!words.size) return undefined
  const near = history.filter((expense) => {
    const theirs = meaningfulWords(normalizeDescription(expense.description))
    if (!theirs.length) return false
    return theirs.every((word) => words.has(word)) || [...words].every((word) => theirs.includes(word))
  })
  return near.length ? mostUsed(near) : undefined
}

// The Category given most often; on a tie, the most recent, since history
// comes newest first.
function mostUsed(expenses: readonly CategorizedExpense[]): CategoryName | undefined {
  const counts = new Map<CategoryName, number>()
  for (const { category } of expenses) counts.set(category, (counts.get(category) ?? 0) + 1)
  let best: CategoryName | undefined
  for (const { category } of expenses) {
    if (best === undefined || (counts.get(category) ?? 0) > (counts.get(best) ?? 0)) best = category
  }
  return best
}

// Words and names that give the Category away, in Spanish and with the shops
// common in Colombia and Latin America. Written already normalized.
const keywords: Record<Exclude<CategoryName, 'other'>, readonly string[]> = {
  food: [
    'almuerzo', 'desayuno', 'cena', 'comida', 'onces', 'merienda', 'brunch', 'mecato', 'snack', 'restaurante',
    'pizza', 'pizzeria', 'hamburguesa', 'perro caliente', 'sushi', 'tacos', 'taqueria', 'arepa', 'empanada',
    'panaderia', 'pan', 'cafe', 'cafeteria', 'tinto', 'helado', 'postre', 'pollo', 'asado', 'parrilla',
    'corrientazo', 'ceviche', 'domicilio', 'rappi', 'ifood', 'didi food', 'uber eats', 'mcdonalds', 'kfc',
    'burger king', 'subway', 'dominos', 'papa johns', 'frisby', 'el corral', 'crepes', 'juan valdez',
    'starbucks', 'oma', 'presto', 'sandwich', 'pasteles', 'tamal', 'lechona',
  ],
  groceries: [
    'mercado', 'supermercado', 'super', 'd1', 'ara', 'exito', 'carulla', 'jumbo', 'olimpica', 'makro',
    'pricesmart', 'colsubsidio', 'isimo', 'oxxo', 'walmart', 'soriana', 'chedraui',
    'lider', 'tottus', 'plaza vea', 'wong', 'mercadona', 'carrefour', 'lidl', 'fruver', 'frutas',
    'verduras', 'carniceria', 'huevos', 'leche', 'despensa', 'abarrotes', 'viveres',
  ],
  transport: [
    'uber', 'taxi', 'didi', 'cabify', 'indrive', 'indriver', 'beat', 'bus', 'buseta', 'colectivo',
    'transmilenio', 'sitp', 'metro', 'metrocable', 'tren', 'pasaje', 'tiquete', 'vuelo', 'avion',
    'aerolinea', 'avianca', 'latam', 'wingo', 'jetsmart', 'volaris', 'aeromexico', 'gasolina',
    'gasolinera', 'combustible', 'tanqueo', 'diesel', 'peaje', 'parqueadero', 'parqueo',
    'estacionamiento', 'soat', 'tecnomecanica', 'lancha', 'ferry', 'moto', 'bicicleta', 'patineta',
  ],
  lodging: [
    'hotel', 'hostal', 'hostel', 'airbnb', 'booking', 'alojamiento', 'posada', 'cabana', 'glamping',
    'motel', 'apartahotel', 'finca', 'hospedaje',
  ],
  housing: [
    'arriendo', 'alquiler', 'renta', 'administracion', 'hipoteca', 'predial', 'condominio', 'casa',
    'apartamento', 'muebles', 'reparacion', 'plomero', 'cerrajero', 'empleada', 'aseo',
  ],
  utilities: [
    'luz', 'energia', 'electricidad', 'agua', 'acueducto', 'gas', 'internet', 'wifi', 'telefono',
    'celular', 'claro', 'movistar', 'tigo', 'wom', 'etb', 'epm', 'enel', 'codensa', 'vanti', 'servicios',
  ],
  entertainment: [
    'cine', 'cinemark', 'cine colombia', 'cinepolis', 'netflix', 'spotify', 'disney', 'hbo', 'prime video',
    'youtube', 'apple tv', 'teatro', 'concierto', 'boleta', 'entrada', 'fiesta', 'rumba', 'discoteca', 'bar',
    'cerveza', 'trago', 'karaoke', 'bolos', 'museo', 'parque', 'juego', 'videojuego', 'playstation', 'xbox',
    'steam', 'nintendo', 'partido', 'futbol', 'tejo', 'paseo', 'tour', 'excursion',
  ],
  shopping: [
    'ropa', 'zapatos', 'tenis', 'camisa', 'pantalon', 'vestido', 'falabella', 'zara', 'amazon',
    'mercado libre', 'mercadolibre', 'shein', 'temu', 'aliexpress', 'ktronix', 'alkosto', 'homecenter',
    'ikea', 'centro comercial', 'electrodomestico', 'computador', 'audifonos',
  ],
  health: [
    'farmacia', 'drogueria', 'medicina', 'medicamento', 'medico', 'doctor', 'consulta', 'eps',
    'odontologo', 'dentista', 'laboratorio', 'examen', 'hospital', 'clinica', 'gimnasio', 'gym',
    'cruz verde', 'farmatodo', 'la rebaja', 'locatel', 'optica', 'terapia', 'psicologo', 'vacuna',
  ],
  gifts: [
    'regalo', 'cumpleanos', 'cumple', 'detalle', 'flores', 'aniversario', 'navidad', 'amigo secreto',
    'baby shower', 'donacion', 'ofrenda',
  ],
}

// Every keyword to its Category. Two-word names come first, so that «mercado
// libre» wins over «mercado».
const byKeyword = new Map<string, CategoryName>()
for (const [category, words] of Object.entries(keywords) as [CategoryName, readonly string[]][]) {
  for (const word of words) byKeyword.set(word, category)
}

function fromKeywords(text: string): CategoryName | undefined {
  const words = text.split(' ')
  for (let i = 0; i < words.length - 1; i++) {
    const pair = byKeyword.get(`${words[i]} ${words[i + 1]}`)
    if (pair) return pair
  }
  for (const word of words) {
    const found = byKeyword.get(word) ?? byKeyword.get(singular(word))
    if (found) return found
  }
  return undefined
}

// 'tiquetes' and 'pasajes' read as their keyword in the singular.
function singular(word: string): string {
  if (word.endsWith('es') && word.length > 4) return byKeyword.has(word.slice(0, -2)) ? word.slice(0, -2) : word.slice(0, -1)
  return word.endsWith('s') ? word.slice(0, -1) : word
}
