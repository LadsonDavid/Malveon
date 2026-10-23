Rails.application.routes.draw do
  post '/refund', to: 'orders#refund'
  get '/cancel-order-info', to: 'orders#cancel_info' # unrelated endpoint - not the same path the caller actually calls
  post '/settings', to: 'settings#update' # registered as POST, but the caller uses PUT
  get '/profile', to: 'profile#show' # nothing calls this - stays NOT TESTED, never guessed unreachable
end
