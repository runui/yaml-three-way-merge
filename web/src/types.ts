export type AlgorithmID = "intent" | "direct" | "rebase";

export interface Algorithm {
  id: AlgorithmID;
  name: string;
  description: string;
  output_type: "effective" | "override";
}

export interface FixtureSummary {
  id: string;
  field: string;
  class: string;
  scenario: string;
  description: string;
  pair_semantics?: string;
}

export interface FixtureDetail {
  metadata: FixtureSummary;
  previous_base: string;
  user_override: string;
  target_base: string;
  expected: string;
}

export interface FixturePage {
  items: FixtureSummary[];
  total: number;
  page: number;
  page_size: number;
}

interface ChangeSummary {
  added: number;
  modified: number;
  removed: number;
  paths?: string[];
}

export interface MergeDiagnostics {
  user: ChangeSummary;
  upstream: ChangeSummary;
  conflicts: {
    write_write: number;
    write_remove: number;
    remove_write: number;
    both_remove: number;
  };
  independent_user: number;
  independent_upstream: number;
  origin_rewrites: number;
  user_replay_valid: boolean;
  upstream_replay_valid: boolean;
}

export interface MergeResponse {
  previous_effective: string;
  new_override: string;
  new_effective: string;
  matches_expected?: boolean;
  diagnostics?: MergeDiagnostics;
}

export interface APIError {
  error: { stage: string; message: string };
}
