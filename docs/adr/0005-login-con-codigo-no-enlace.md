# Inicio de sesión con código por email, no con enlace mágico

Se inicia sesión con un código de 6 dígitos enviado por email o con Google; no hay contraseñas. No usamos enlaces mágicos porque en iOS la PWA instalada no comparte almacenamiento con Safari: el enlace se abre en Safari, la sesión se queda allí y la PWA sigue sin iniciar sesión.
