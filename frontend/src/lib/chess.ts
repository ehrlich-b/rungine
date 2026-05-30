export const STARTING_FEN =
  'rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1';

/** A FEN piece char is one of "PNBRQKpnbrqk"; empty squares are encoded as null. */
export type PieceChar = 'P' | 'N' | 'B' | 'R' | 'Q' | 'K' | 'p' | 'n' | 'b' | 'r' | 'q' | 'k';
export type Square = PieceChar | null;

export interface Position {
  /** 8x8 board, board[0] is rank 8 (black side), board[7] is rank 1. board[r][f] where f=0 is the a-file. */
  board: Square[][];
  turn: 'w' | 'b';
  castling: string;
  enPassant: string;
  halfMove: number;
  fullMove: number;
}

const PIECES: Record<PieceChar, string> = {
  K: '♔',
  Q: '♕',
  R: '♖',
  B: '♗',
  N: '♘',
  P: '♙',
  k: '♚',
  q: '♛',
  r: '♜',
  b: '♝',
  n: '♞',
  p: '♟',
};

export function pieceGlyph(piece: PieceChar): string {
  return PIECES[piece];
}

export function parseFEN(fen: string): Position {
  const parts = fen.trim().split(/\s+/);
  if (parts.length < 1) throw new Error('empty FEN');
  const ranks = parts[0].split('/');
  if (ranks.length !== 8) throw new Error(`FEN must have 8 ranks, got ${ranks.length}`);

  const board: Square[][] = [];
  for (const rank of ranks) {
    const row: Square[] = [];
    for (const ch of rank) {
      if (ch >= '1' && ch <= '8') {
        const n = ch.charCodeAt(0) - '0'.charCodeAt(0);
        for (let i = 0; i < n; i++) row.push(null);
      } else if ('PNBRQKpnbrqk'.includes(ch)) {
        row.push(ch as PieceChar);
      } else {
        throw new Error(`invalid FEN piece char: ${ch}`);
      }
    }
    if (row.length !== 8) throw new Error(`rank must have 8 files, got ${row.length}`);
    board.push(row);
  }

  return {
    board,
    turn: (parts[1] === 'b' ? 'b' : 'w') as 'w' | 'b',
    castling: parts[2] ?? '-',
    enPassant: parts[3] ?? '-',
    halfMove: parts[4] ? parseInt(parts[4], 10) : 0,
    fullMove: parts[5] ? parseInt(parts[5], 10) : 1,
  };
}

/** A board arrow overlay: engine PV, user annotation, etc. Consumed by Board's `arrows` prop. */
export type Arrow = {
  /** Source square in UCI notation, e.g. "e2". */
  from: string;
  /** Destination square in UCI notation, e.g. "e4". */
  to: string;
  /** CSS color string. Defaults to the accent color in Board. */
  color?: string;
  /** Multiplier on the default stroke width. */
  weight?: number;
};

/** Convert a UCI move (e.g. "e2e4", "e7e8q") to a board Arrow. Returns null if malformed. */
export function uciToArrow(uci: string, color?: string, weight?: number): Arrow | null {
  if (!uci || uci.length < 4) return null;
  const arrow: Arrow = { from: uci.slice(0, 2), to: uci.slice(2, 4) };
  if (color) arrow.color = color;
  if (weight !== undefined) arrow.weight = weight;
  return arrow;
}

/**
 * Normalize an engine score to white's POV. UCI engines report from the
 * side-to-move's POV, so a black move's score must be negated.
 */
export function whitePov(value: number, side: 'w' | 'b'): number {
  return side === 'b' ? -value : value;
}

/** Convert a UCI square (e.g. "e4") to {file, rank} indices, where file 0=a, rank 0=8. */
export function squareToCoords(sq: string): { file: number; rank: number } {
  const file = sq.charCodeAt(0) - 'a'.charCodeAt(0);
  const rank = 8 - parseInt(sq[1] ?? '1', 10);
  return { file, rank };
}

/** Convert board indices back to UCI square. file 0=a, rank 0=8 corresponds to a8. */
export function coordsToSquare(file: number, rank: number): string {
  return String.fromCharCode('a'.charCodeAt(0) + file) + (8 - rank).toString();
}
