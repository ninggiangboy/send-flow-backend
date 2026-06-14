// ============================================================
// Response Envelope
// ============================================================

data class Meta(val requestId: String)

data class Envelope<T>(val data: T, val meta: Meta)

data class ErrorBody(
    val code: String,
    val message: String,
    val details: Map<String, Any?>? = null,
)

data class ErrorEnvelope(val error: ErrorBody, val meta: Meta)

// ============================================================
// Domain Error (replaces sentinel errors + errors.Is chain)
// ============================================================

sealed interface DomainError {
    val code: String
    val message: String
    val httpStatus: Int
    val details: Map<String, Any?>?
}

// --- Auth errors ---
sealed interface AuthError : DomainError {
    override val code: String get() = "auth.${this::class.simpleName?.toSnakeCase()}"
    override val details: Map<String, Any?>? get() = null

    data object InvalidCredentials : AuthError {
        override val message = "invalid credentials"
        override val httpStatus = 401
    }
    data object EmailAlreadyExists : AuthError {
        override val message = "email already registered"
        override val httpStatus = 409
        override val details = mapOf("field" to "email")
    }
    data object Unauthorized : AuthError {
        override val message = "invalid token"
        override val httpStatus = 401
    }
    data object PasswordPolicy : AuthError {
        override val message = "password policy violation"
        override val httpStatus = 422
    }
    data object RateLimited : AuthError {
        override val message = "rate limited"
        override val httpStatus = 429
    }
    data object SessionNotOwned : AuthError {
        override val message = "session not owned"
        override val httpStatus = 403
    }
}

// --- Campaign errors ---
sealed interface CampaignError : DomainError {
    data object ReadDenied : CampaignError {
        override val code = "campaign.read_denied"
        override val message = "read denied"
        override val httpStatus = 403
        override val details = null
    }
    data object WriteDenied : CampaignError {
        override val code = "campaign.write_denied"
        override val message = "write denied"
        override val httpStatus = 403
        override val details = null
    }
    data object NotFound : CampaignError {
        override val code = "campaign.not_found"
        override val message = "campaign not found"
        override val httpStatus = 404
        override val details = null
    }
    data object PayloadInvalid : CampaignError {
        override val code = "campaign.payload_invalid"
        override val message = "invalid payload"
        override val httpStatus = 422
        override val details = null
    }
    data object InvalidStateTransition : CampaignError {
        override val code = "campaign.invalid_state_transition"
        override val message = "invalid state transition"
        override val httpStatus = 409
        override val details = null
    }
    data object PermissionDenied : CampaignError {
        override val code = "auth.permission_denied"
        override val message = "permission denied"
        override val httpStatus = 403
        override val details = null
    }
}

// Fallback for unmapped errors
data class InternalError(
    override val message: String = "internal error",
) : DomainError {
    override val code = "internal.error"
    override val httpStatus = 500
    override val details = null
}

// ============================================================
// Use case result (replaces (*T, error) return pattern)
// ============================================================

sealed interface UseCaseResult<out T> {
    data class Success<T>(val data: T) : UseCaseResult<T>
    data class Failure(val error: DomainError) : UseCaseResult<Nothing>
}

// ============================================================
// Cursor-based pagination (replaces XxxListResult pattern)
// ============================================================

data class CursorPage<T>(
    val items: List<T>,
    val nextCursor: String?,
) {
    companion object {
        fun <T> empty() = CursorPage(emptyList(), null)

        fun <T> single(item: T, nextCursor: String? = null) =
            CursorPage(listOf(item), nextCursor)
    }

    val hasMore: Boolean get() = nextCursor != null
}

// ============================================================
// Non-retryable error marker (for worker/async)
// ============================================================

interface NonRetryable

class NonRetryableException(
    override val message: String,
    cause: Throwable? = null,
) : RuntimeException(message, cause), NonRetryable

// ============================================================
// Response helpers (replaces writeEnvelope / writeError)
// ============================================================

object ResponseWriter {
    private val objectMapper = jacksonObjectMapper()

    fun <T> ok(data: T, requestId: String): String =
        objectMapper.writeValueAsString(
            Envelope(data = data, meta = Meta(requestId = requestId))
        )

    fun error(err: DomainError, requestId: String, status: Int = err.httpStatus): String =
        objectMapper.writeValueAsString(
            ErrorEnvelope(
                error = ErrorBody(
                    code = err.code,
                    message = err.message,
                    details = err.details,
                ),
                meta = Meta(requestId = requestId),
            )
        )

    fun internalError(requestId: String): String =
        error(InternalError(), requestId)
}

// ============================================================
// HTTP status mapping (replaces writeXxxErr switch)
// ============================================================

fun DomainError.toHttpStatus(): Int = when (this) {
    is AuthError.EmailAlreadyExists -> 409
    is AuthError.InvalidCredentials -> 401
    is AuthError.Unauthorized -> 401
    is AuthError.PasswordPolicy -> 422
    is AuthError.RateLimited -> 429
    is AuthError.SessionNotOwned -> 403
    is CampaignError.ReadDenied -> 403
    is CampaignError.WriteDenied -> 403
    is CampaignError.NotFound -> 404
    is CampaignError.PayloadInvalid -> 422
    is CampaignError.InvalidStateTransition -> 409
    is CampaignError.PermissionDenied -> 403
    is InternalError -> 500
}

// ============================================================
// Utility
// ============================================================

private fun String.toSnakeCase(): String =
    replace(Regex("([a-z])([A-Z])"), "$1_$2")
        .lowercase()
