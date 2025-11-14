"use client";

import { useMemo } from "react";

type DonationQRCodeProps = {
  value: string;
  label: string;
  size?: number;
};

const MATRIX_SIZE = 21;

function createSeed(value: string): number {
  let hash = 0x811c9dc5; // FNV-1a 32-bit offset basis
  for (let i = 0; i < value.length; i += 1) {
    hash ^= value.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193);
  }
  return hash >>> 0;
}

function mulberry32(seed: number): () => number {
  return () => {
    let t = (seed += 0x6d2b79f5);
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function applyFinderPattern(matrix: boolean[][], top: number, left: number) {
  for (let y = 0; y < 7; y += 1) {
    for (let x = 0; x < 7; x += 1) {
      const outer = x === 0 || x === 6 || y === 0 || y === 6;
      const inner = x >= 2 && x <= 4 && y >= 2 && y <= 4;
      matrix[top + y][left + x] = outer || inner;
    }
  }
}

function applyTimingPatterns(matrix: boolean[][]) {
  for (let i = 8; i < MATRIX_SIZE - 8; i += 1) {
    const bit = i % 2 === 0;
    matrix[6][i] = bit;
    matrix[i][6] = bit;
  }
}

function generateMatrix(value: string): boolean[][] {
  const matrix: boolean[][] = Array.from({ length: MATRIX_SIZE }, () =>
    Array(MATRIX_SIZE).fill(false)
  );

  // Place finder patterns and separators.
  applyFinderPattern(matrix, 0, 0);
  applyFinderPattern(matrix, 0, MATRIX_SIZE - 7);
  applyFinderPattern(matrix, MATRIX_SIZE - 7, 0);

  // Clear separator spaces around finder patterns.
  const clear = (top: number, left: number) => {
    for (let y = -1; y <= 7; y += 1) {
      for (let x = -1; x <= 7; x += 1) {
        const posY = top + y;
        const posX = left + x;
        if (
          posY >= 0 &&
          posY < MATRIX_SIZE &&
          posX >= 0 &&
          posX < MATRIX_SIZE &&
          (y === -1 || y === 7 || x === -1 || x === 7)
        ) {
          matrix[posY][posX] = false;
        }
      }
    }
  };
  clear(0, 0);
  clear(0, MATRIX_SIZE - 7);
  clear(MATRIX_SIZE - 7, 0);

  applyTimingPatterns(matrix);

  const seed = createSeed(value);
  const rand = mulberry32(seed);

  for (let y = 0; y < MATRIX_SIZE; y += 1) {
    for (let x = 0; x < MATRIX_SIZE; x += 1) {
      const isFinderZone =
        (x < 8 && y < 8) ||
        (x >= MATRIX_SIZE - 8 && y < 8) ||
        (x < 8 && y >= MATRIX_SIZE - 8);
      const isTiming = x === 6 || y === 6;

      if (isFinderZone || isTiming) {
        continue;
      }
      matrix[y][x] = rand() > 0.5;
    }
  }

  return matrix;
}

export function DonationQRCode({ value, label, size = 80 }: DonationQRCodeProps) {
  const matrix = useMemo(() => generateMatrix(value), [value]);

  return (
    <svg
      viewBox={`0 0 ${MATRIX_SIZE} ${MATRIX_SIZE}`}
      width={size}
      height={size}
      role="img"
      aria-label={label}
      focusable="false"
      className="donation-item__qr-code"
    >
      <rect width="100%" height="100%" fill="white" rx="1.2" />
      <g fill="currentColor" shapeRendering="crispEdges">
        {matrix.map((row, y) =>
          row.map((active, x) =>
            active ? (
              <rect key={`${x}-${y}`} x={x} y={y} width={1} height={1} />
            ) : null
          )
        )}
      </g>
    </svg>
  );
}
