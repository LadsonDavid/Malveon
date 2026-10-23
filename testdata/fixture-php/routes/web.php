<?php

Route::post('/refund', [OrderController::class, 'refund']);
Route::get('/cancel-order-info', [OrderController::class, 'cancelInfo']); // unrelated endpoint - not the same path the caller actually calls
Route::post('/settings', [SettingsController::class, 'update']); // registered as POST, but the caller uses PUT
Route::get('/profile', [ProfileController::class, 'show']); // nothing calls this - stays NOT TESTED, never guessed unreachable
