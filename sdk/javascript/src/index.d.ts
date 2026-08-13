export interface PoDLClientOptions { apiKey?: string; fetchImpl?: typeof fetch }
export interface ContractCall { address: string; function: string; args?: string[]; from?: string; value?: string; readOnly?: boolean }
export interface SuitabilityAnswers {
  loss_tolerance_bps: number;
  investment_horizon_days: number;
  liquidity_need_days: number;
  defi_experience_years: number;
  accepts_impermanent_loss: boolean;
  accepts_smart_contract_risk: boolean;
}
export class PoDLClient {
  constructor(baseUrl: string, options?: PoDLClientOptions);
  request(path: string, options?: RequestInit): Promise<any>;
  protocolStatus(): Promise<any>;
  mainnetReadiness(): Promise<any>;
  suitability(answers: SuitabilityAnswers): Promise<any>;
  faucet(address: string): Promise<any>;
  balance(address: string): Promise<any>;
  transaction(hash: string): Promise<any>;
  swapQuote(input: { amountIn: string | number; tokenIn: string; tokenOut: string; factory: string }): Promise<any>;
  bestRoute(input: { router: string; amountIn: string | number; tokenIn: string; tokenOut: string }): Promise<any>;
  vaultAccounting(vault: string): Promise<any>;
  vaultWithdrawalReceipt(vault: string, id: string | number): Promise<any>;
  concentratedPosition(pool: string, id: string | number): Promise<any>;
  mintConcentratedPosition(input: { pool: string; from: string; lowerSqrtX18: string | number; upperSqrtX18: string | number; amount0: string | number; amount1: string | number }): Promise<any>;
  transferConcentratedPosition(input: { pool: string; from: string; id: string | number; to: string }): Promise<any>;
  collectConcentratedPositionFees(input: { pool: string; from: string; id: string | number }): Promise<any>;
  burnConcentratedPosition(input: { pool: string; from: string; id: string | number }): Promise<any>;
  contractCall(input: ContractCall): Promise<any>;
  sendSignedTransaction(transaction: unknown): Promise<any>;
  submitOracleUpdate(signedTransaction: unknown): Promise<any>;
  submitGovernanceAction(signedTransaction: unknown): Promise<any>;
}
export class PoDLError extends Error { status: number; details: any }
export function controlSigningPayload(transaction: any): string;
export function controlTransactionDigest(transaction: any): Promise<Uint8Array>;
