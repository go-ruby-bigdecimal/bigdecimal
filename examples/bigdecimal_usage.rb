# frozen_string_literal: true

require "bigdecimal"

# Exact decimal arithmetic: 0.1 + 0.2 is exactly 0.3, not 0.30000000000000004.
sum = BigDecimal("0.1") + BigDecimal("0.2")
puts sum.to_s("F")                                     # => 0.3

# The default to_s renders MRI's canonical scientific notation.
puts BigDecimal("1.5").to_s                             # => 0.15e1

# Division with an explicit precision (digits), and integer power.
puts BigDecimal("22").div(BigDecimal("7"), 10).to_s("F") # => 3.142857143
puts (BigDecimal("2") ** 10).to_s("F")                   # => 1024.0

# Rounding at a decimal place, with an explicit rounding mode.
puts BigDecimal("3.14159").round(2).to_s("F")            # => 3.14
puts BigDecimal("2.5").round(0, BigDecimal::ROUND_HALF_EVEN).to_s("F") # => 2.0

# floor / ceil to a number of decimal places.
puts BigDecimal("3.14159").floor(2).to_s("F")            # => 3.14
puts BigDecimal("3.14159").ceil(2).to_s("F")             # => 3.15

# Euclidean divmod returns the [quotient, remainder] pair.
p BigDecimal("7").divmod(BigDecimal("3")).map { |x| x.to_s("F") } # => ["2.0", "1.0"]

# Conversions and the special values.
puts BigDecimal("-3.75").abs.to_s("F")                   # => 3.75
puts BigDecimal("10").to_i                               # => 10
puts BigDecimal("Infinity").to_s                         # => Infinity
