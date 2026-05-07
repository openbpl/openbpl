import { defineRule } from "@openbpl/sdk";
import { readFileSync } from "node:fs";
import { join, basename, extname } from "node:path";

/**
 * Perceptual hash comparison for favicons.
 *
 * This rule extracts the favicon URL from the captured page HTML,
 * fetches it, and compares it against reference brand images using
 * average hash comparison.
 *
 * Note: For a production implementation you'd want a proper perceptual
 * hash library. This uses a simplified average hash approach.
 */

const THRESHOLD = 5; // max hamming distance

function extractFaviconURL(html: string, domain: string): string | null {
  const linkMatch = html.match(
    /<link[^>]+rel=["'](?:shortcut )?icon["'][^>]*>/i
  );
  if (linkMatch) {
    const hrefMatch = linkMatch[0].match(/href=["']([^"']+)["']/i);
    if (hrefMatch && hrefMatch[1]) {
      const u = hrefMatch[1];
      if (u.startsWith("//")) return `https:${u}`;
      if (u.startsWith("/")) return `https://${domain}${u}`;
      if (u.startsWith("http")) return u;
      return `https://${domain}/${u}`;
    }
  }
  return `https://${domain}/favicon.ico`;
}

async function fetchImage(url: string): Promise<Buffer | null> {
  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 5000);
    const resp = await fetch(url, { signal: controller.signal });
    clearTimeout(timeout);
    if (!resp.ok) return null;
    const arrayBuf = await resp.arrayBuffer();
    return Buffer.from(arrayBuf);
  } catch {
    return null;
  }
}

/**
 * Simple average hash: resize to 8x8 conceptually by sampling,
 * compute mean, produce 64-bit hash. This is a simplified version.
 * For production use, consider a native image hashing library.
 */
function simpleImageHash(data: Buffer): bigint | null {
  // Detect PNG by magic bytes
  if (data[0] === 0x89 && data[1] === 0x50) {
    // Use raw byte distribution as a simplified fingerprint
    const samples = new Uint8Array(64);
    const step = Math.max(1, Math.floor(data.length / 64));
    for (let i = 0; i < 64; i++) {
      samples[i] = data[Math.min(i * step, data.length - 1)];
    }
    const mean =
      samples.reduce((sum, val) => sum + val, 0) / samples.length;
    let hash = 0n;
    for (let i = 0; i < 64; i++) {
      if (samples[i] >= mean) {
        hash |= 1n << BigInt(i);
      }
    }
    return hash;
  }
  return null;
}

function hammingDistance(a: bigint, b: bigint): number {
  let xor = a ^ b;
  let dist = 0;
  while (xor > 0n) {
    dist += Number(xor & 1n);
    xor >>= 1n;
  }
  return dist;
}

export default defineRule({
  name: "favicon-match",
  description:
    "Compares page favicon against reference brand images using perceptual hashing",

  async evaluate({ evidence, brand }) {
    if (!brand.images || brand.images.length === 0) return null;

    const faviconURL = extractFaviconURL(evidence.html, evidence.domain);
    if (!faviconURL) return null;

    const faviconData = await fetchImage(faviconURL);
    if (!faviconData || faviconData.length < 100) return null;

    const faviconHash = simpleImageHash(faviconData);
    if (faviconHash === null) return null;

    for (const imagePath of brand.images) {
      try {
        const refData = readFileSync(imagePath);
        const refHash = simpleImageHash(refData);
        if (refHash === null) continue;

        const dist = hammingDistance(faviconHash, refHash);
        if (dist <= THRESHOLD) {
          const confidence = 1.0 - dist / (THRESHOLD + 1);
          const name = basename(imagePath, extname(imagePath));
          return {
            name: "favicon-match",
            confidence,
            detail: `matches ${name} (hamming=${dist})`,
          };
        }
      } catch {
        continue;
      }
    }

    return null;
  },
});
