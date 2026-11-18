# Circuit Lab

A visual Arduino Uno workbench built into the simulator (sidebar: **Circuit Lab**).

## Using it
1. Pick an **example circuit** or press *Add part* to place components.
2. **Wire**: press on a gold pin and drag to another pin. Drag parts to move them. Select a wire or part and press *Delete*.
3. Edit the **sketch** (Arduino C/C++ subset) and press **Run**. Hover a pin to read its voltage.
4. Change inputs live: LDR light, DHT11 temperature/humidity, PIR motion, HC-SR04 distance, potentiometer, buttons.
5. Save designs to the workspace (`circuits/`) with *Save*; reopen them with *Open*.

Parts: LED (5 colours), resistor, push button, potentiometer, LDR, DHT11, PIR, HC-SR04, buzzer.

## What is simulated
- Nets from your wires, then node voltages (resistor networks, LDR/pot dividers, pull-ups, LED forward drop).
- LEDs glow by current; too much current **burns** them (add a 220 ohm resistor).
- Floating inputs read random values and are reported; short circuits are reported.
- DHT11 and HC-SR04 only answer when wired to the pin used in the sketch and powered.
- PWM (`analogWrite`) dims LEDs on pins 3, 5, 6, 9, 10, 11; `tone()` sounds the buzzer.
- Virtual time: `delay()`, `millis()` and `micros()` follow simulated time; speeds 1x to 10x.

## Sketch language (supported subset)
`setup()`/`loop()`, functions (recursion allowed), `int long byte char bool float String`, arrays, `#define`, `const`,
`if/else for while do switch`, `Serial.begin/print/println/write`, `pinMode digitalWrite digitalRead analogRead analogWrite`,
`delay delayMicroseconds millis micros`, `tone noTone pulseIn`, `map constrain min max abs pow sqrt sq sin cos ...`,
`random randomSeed`, `DHT dht(pin, DHT11)` with `readTemperature/readHumidity`, `String` methods `length toInt toFloat`.
Integer sizes follow the Uno (`int` is 16 bit and wraps).

Not supported yet: pointers, structs/classes, interrupts, `Servo`, `LiquidCrystal`, I2C/SPI, EEPROM, other libraries,
multi-dimensional arrays, function-like `#define` macros. ESP32 and Nano 33 BLE boards are on the roadmap.

## Lab 2 link
Open **Lab 2 -> Open the Lab 2 wiring exercise**: wire DHT11 + PIR + LDR yourself. Five clean `T,H,M,L` lines
mark the Lab 2 student task *Circuit built visually and run* as done and appear in your PDF report.

## Tests
`node --test web-tests/*.test.js` covers the interpreter (types, timing, errors) and the electrical model.
