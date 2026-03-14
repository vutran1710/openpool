CREATE OR REPLACE FUNCTION create_like_and_maybe_match(
    p_liker_id UUID,
    p_liked_id UUID
) RETURNS TABLE (
    like_id  UUID,
    match_id UUID,
    conv_id  UUID,
    is_match BOOLEAN
) LANGUAGE plpgsql AS $$
DECLARE
    v_like_id  UUID;
    v_match_id UUID := NULL;
    v_conv_id  UUID := NULL;
    v_is_match BOOLEAN := false;
    v_user_a   UUID;
    v_user_b   UUID;
BEGIN
    IF p_liker_id = p_liked_id THEN
        RAISE EXCEPTION 'Cannot like yourself';
    END IF;

    INSERT INTO likes (liker_id, liked_id)
    VALUES (p_liker_id, p_liked_id)
    RETURNING id INTO v_like_id;

    IF EXISTS (
        SELECT 1 FROM likes WHERE liker_id = p_liked_id AND liked_id = p_liker_id
    ) THEN
        v_is_match := true;

        IF p_liker_id < p_liked_id THEN
            v_user_a := p_liker_id;
            v_user_b := p_liked_id;
        ELSE
            v_user_a := p_liked_id;
            v_user_b := p_liker_id;
        END IF;

        INSERT INTO matches (user_a, user_b)
        VALUES (v_user_a, v_user_b)
        ON CONFLICT (user_a, user_b) DO NOTHING
        RETURNING id INTO v_match_id;

        IF v_match_id IS NOT NULL THEN
            INSERT INTO conversations (match_id)
            VALUES (v_match_id)
            RETURNING id INTO v_conv_id;
        END IF;
    END IF;

    RETURN QUERY SELECT v_like_id, v_match_id, v_conv_id, v_is_match;
END;
$$;

CREATE OR REPLACE FUNCTION discover_profiles(
    p_requester_id UUID,
    p_city         TEXT DEFAULT NULL,
    p_interest     TEXT DEFAULT NULL,
    p_limit        INT DEFAULT 1
) RETURNS SETOF profiles_index
LANGUAGE sql STABLE AS $$
    SELECT pi.*
    FROM profiles_index pi
    WHERE pi.discoverable = true
      AND pi.user_id <> p_requester_id
      AND NOT EXISTS (
          SELECT 1 FROM likes WHERE liker_id = p_requester_id AND liked_id = pi.user_id
      )
      AND NOT EXISTS (
          SELECT 1 FROM matches
          WHERE user_a = LEAST(p_requester_id, pi.user_id)
            AND user_b = GREATEST(p_requester_id, pi.user_id)
      )
      AND (p_city IS NULL OR pi.city ILIKE p_city)
      AND (p_interest IS NULL OR p_interest = ANY(pi.interests))
    ORDER BY random()
    LIMIT p_limit;
$$;
