export type LoadState = {
  isLoading: boolean;
  isError: boolean;
  error: unknown;
};


export function queryState(query: { isLoading: boolean; isError: boolean; error: unknown }): LoadState {
  return {
    isLoading: query.isLoading,
    isError: query.isError,
    error: query.error,
  };
}
