#!/usr/bin/env node

let input = "";
process.stdin.on("data", chunk => {
  input += chunk;
});

process.stdin.on("end", () => {
  const req = JSON.parse(input || "{}");
  const text = String((req.data || {}).text || "");
  process.stdout.write(JSON.stringify({
    result: {
      reversed: Array.from(text).reverse().join(""),
      length: Array.from(text).length
    }
  }));
});
